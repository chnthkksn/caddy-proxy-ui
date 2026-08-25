// Package trafficlog tails the JSON access log Caddy writes (see
// caddyconfig.Build's "access" logger) and aggregates it into a rolling
// 24-hour hourly count plus a capped list of recent requests. It never
// persists this state — a caddy-ui restart just re-scans the log file from
// its start, which is honest (a briefly-empty window right after start is
// truthful, not faked) and cheap (only real traffic already on disk).
package trafficlog

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	maxRecent       = 200
	pruneAfterHours = 30 // a little more than the 24h window we ever report, so it's always fully covered
)

type Entry struct {
	Time     time.Time
	Domain   string
	Status   int
	Duration float64 // seconds
}

type HourBucket struct {
	HourStart time.Time
	Requests  int
	Errors    int // status >= 400
}

type Snapshot struct {
	Hourly    []HourBucket // 24 entries, oldest to newest, zero-filled for quiet hours
	Recent    []Entry      // most recent first, capped at maxRecent
	Total24h  int
	Errors24h int
	// ByDomain counts requests per host over the same 24h window, so the
	// hosts table can show a real per-host request count rather than a
	// placeholder.
	ByDomain map[string]int
}

// bucket is the internal per-hour tally. It carries a per-domain breakdown
// that HourBucket deliberately doesn't expose — the 24h rollup is what
// callers want, not 24 separate per-hour maps.
type bucket struct {
	hourStart time.Time
	requests  int
	errors    int
	domains   map[string]int
}

type Tailer struct {
	mu     sync.Mutex
	path   string
	offset int64
	hours  map[int64]*bucket // key: unix hour (ts / 3600)
	recent []Entry           // oldest first
}

func New(path string) *Tailer {
	return &Tailer{path: path, hours: make(map[int64]*bucket)}
}

// Snapshot re-scans whatever is new in the log file since the last call,
// then returns the current rolling window as of now.
func (t *Tailer) Snapshot(now time.Time) Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.scanLocked()
	return t.buildSnapshotLocked(now)
}

func (t *Tailer) scanLocked() {
	f, err := os.Open(t.path)
	if err != nil {
		// No log file yet (e.g. nothing has synced to Caddy, or no traffic
		// has landed since caddy-ui's own last start) — an empty window is
		// the honest answer, not an error.
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return
	}
	if info.Size() < t.offset {
		// The file is smaller than where we last left off — it was rotated
		// or truncated. Re-scan it from the start rather than crashing or
		// seeking past EOF.
		t.offset = 0
	}
	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return
	}

	reader := bufio.NewReaderSize(f, 64*1024)
	for {
		line, err := reader.ReadString('\n')
		if err == nil {
			t.ingestLocked(strings.TrimRight(line, "\n"))
			t.offset += int64(len(line))
			continue
		}
		// err != nil: either EOF or a real read error. Either way, any
		// partial trailing line (Caddy still mid-write) is deliberately
		// left unconsumed — the offset doesn't advance past it, so the next
		// scan picks it up once the newline lands.
		break
	}
}

type rawLogLine struct {
	Logger  string  `json:"logger"`
	TS      float64 `json:"ts"`
	Request struct {
		Host string `json:"host"`
	} `json:"request"`
	Status   int     `json:"status"`
	Duration float64 `json:"duration"`
}

func (t *Tailer) ingestLocked(line string) {
	if line == "" {
		return
	}
	var raw rawLogLine
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return
	}
	// Defensive: the logger's "include" filter should already guarantee
	// this, but don't trust foreign log lines if that ever changes.
	if !strings.HasPrefix(raw.Logger, "http.log.access") {
		return
	}

	ts := time.Unix(0, int64(raw.TS*float64(time.Second)))
	entry := Entry{
		Time:     ts,
		Domain:   hostWithoutPort(raw.Request.Host),
		Status:   raw.Status,
		Duration: raw.Duration,
	}

	t.recent = append(t.recent, entry)
	if len(t.recent) > maxRecent {
		t.recent = t.recent[len(t.recent)-maxRecent:]
	}

	hourKey := ts.Unix() / 3600
	b := t.hours[hourKey]
	if b == nil {
		b = &bucket{
			hourStart: time.Unix(hourKey*3600, 0).UTC(),
			domains:   make(map[string]int),
		}
		t.hours[hourKey] = b
	}
	b.requests++
	if raw.Status >= 400 {
		b.errors++
	}
	if entry.Domain != "" {
		b.domains[entry.Domain]++
	}
}

// hostWithoutPort normalises the log's request.host so it can be matched
// against a stored host's domain. Caddy logs the Host header verbatim, which
// carries a port whenever it isn't the scheme default — so anyone running
// Caddy on a non-standard port would otherwise get no per-host counts at all.
func hostWithoutPort(host string) string {
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	// No port present (the common case), or an IPv6 literal without one.
	return strings.Trim(host, "[]")
}

func (t *Tailer) pruneLocked(now time.Time) {
	cutoff := now.Add(-pruneAfterHours*time.Hour).Unix() / 3600
	for k := range t.hours {
		if k < cutoff {
			delete(t.hours, k)
		}
	}
}

func (t *Tailer) buildSnapshotLocked(now time.Time) Snapshot {
	t.pruneLocked(now)

	nowHour := now.Unix() / 3600
	hourly := make([]HourBucket, 0, 24)
	byDomain := make(map[string]int)
	var total, errs int
	for i := 23; i >= 0; i-- {
		key := nowHour - int64(i)
		b := t.hours[key]
		if b == nil {
			hourly = append(hourly, HourBucket{HourStart: time.Unix(key*3600, 0).UTC()})
			continue
		}
		hourly = append(hourly, HourBucket{
			HourStart: b.hourStart,
			Requests:  b.requests,
			Errors:    b.errors,
		})
		total += b.requests
		errs += b.errors
		for domain, n := range b.domains {
			byDomain[domain] += n
		}
	}

	recent := make([]Entry, len(t.recent))
	copy(recent, t.recent)
	for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
		recent[i], recent[j] = recent[j], recent[i]
	}

	return Snapshot{
		Hourly:    hourly,
		Recent:    recent,
		Total24h:  total,
		Errors24h: errs,
		ByDomain:  byDomain,
	}
}
