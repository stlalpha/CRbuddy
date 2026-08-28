package tally

import "github.com/stlalpha/prpal/internal/ghclient"

// Tally counts CodeRabbit review threads.
type Tally struct {
	Total    int
	Resolved int
	Open     int // always Total - Resolved
}

// Count tallies the given threads: Total = len, Resolved = count of IsResolved, Open = Total - Resolved.
func Count(threads []ghclient.Thread) Tally {
	total := len(threads)
	resolved := 0
	for _, th := range threads {
		if th.IsResolved {
			resolved++
		}
	}
	return Tally{
		Total:    total,
		Resolved: resolved,
		Open:     total - resolved,
	}
}

// Add returns the element-wise sum of t and o.
func (t Tally) Add(o Tally) Tally {
	return Tally{
		Total:    t.Total + o.Total,
		Resolved: t.Resolved + o.Resolved,
		Open:     t.Open + o.Open,
	}
}

// Done reports Total > 0 && Open == 0 (all threads resolved). Total == 0 returns false.
func (t Tally) Done() bool {
	return t.Total > 0 && t.Open == 0
}
