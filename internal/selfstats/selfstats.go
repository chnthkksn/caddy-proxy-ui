// Package selfstats reports caddy-ui's own real resource usage — no
// simulated or placeholder figures.
package selfstats

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// RSSBytes returns this process's real resident memory usage. On Linux
// (both of caddy-ui's deployment targets — Docker and native systemd — run
// on Linux) it reads /proc/self/status's VmRSS. Elsewhere (e.g. local dev on
// macOS) it falls back to runtime.MemStats.Sys — a different real number
// from the Go runtime itself, rather than showing a fake or zero value.
func RSSBytes() int64 {
	if b, ok := linuxRSS(); ok {
		return b
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Sys)
}

func linuxRSS() (int64, bool) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, false
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, false
		}
		return kb * 1024, true
	}
	return 0, false
}
