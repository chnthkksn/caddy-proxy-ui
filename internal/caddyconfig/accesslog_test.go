package caddyconfig

import "testing"

func TestAccessLogRoll(t *testing.T) {
	cases := []struct {
		budget, size, keep int
		note               string
	}{
		{50, 10, 4, "default: 5 files of 10MB"},
		{10, 2, 4, "small budget still splits"},
		{5, 1, 4, "exactly the file count"},
		{4, 4, 0, "too small to split: one capped file, no history"},
		{1, 1, 0, "minimum"},
		{0, 1, 0, "unset/zero never means unbounded"},
		{-5, 1, 0, "negative never means unbounded"},
	}
	for _, c := range cases {
		size, keep := AccessLog{MaxMB: c.budget}.roll()
		if size != c.size || keep != c.keep {
			t.Errorf("budget %d: got size=%d keep=%d, want size=%d keep=%d (%s)",
				c.budget, size, keep, c.size, c.keep, c.note)
		}
		if total := size * (keep + 1); c.budget >= 1 && total > c.budget {
			t.Errorf("budget %d: worst case %dMB exceeds the budget", c.budget, total)
		}
	}
}
