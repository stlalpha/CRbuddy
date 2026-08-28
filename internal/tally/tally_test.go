package tally

import (
	"testing"

	"github.com/stlalpha/CRbuddy/internal/ghclient"
)

func TestCount(t *testing.T) {
	cases := []struct {
		name    string
		threads []ghclient.Thread
		want    Tally
	}{
		{"empty", nil, Tally{}},
		{
			"all resolved",
			[]ghclient.Thread{{IsResolved: true}, {IsResolved: true}},
			Tally{Total: 2, Resolved: 2, Open: 0},
		},
		{
			"all open",
			[]ghclient.Thread{{IsResolved: false}, {IsResolved: false}, {IsResolved: false}},
			Tally{Total: 3, Resolved: 0, Open: 3},
		},
		{
			"mixed",
			[]ghclient.Thread{{IsResolved: true}, {IsResolved: false}, {IsResolved: true}},
			Tally{Total: 3, Resolved: 2, Open: 1},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Count(c.threads)
			if got != c.want {
				t.Errorf("Count() = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	a := Tally{Total: 3, Resolved: 2, Open: 1}
	b := Tally{Total: 5, Resolved: 1, Open: 4}
	got := a.Add(b)
	want := Tally{Total: 8, Resolved: 3, Open: 5}
	if got != want {
		t.Errorf("Add() = %+v, want %+v", got, want)
	}
}

func TestDone(t *testing.T) {
	cases := []struct {
		name string
		t    Tally
		want bool
	}{
		{"zero total", Tally{}, false},
		{"all resolved", Tally{Total: 4, Resolved: 4, Open: 0}, true},
		{"some open", Tally{Total: 4, Resolved: 3, Open: 1}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.t.Done(); got != c.want {
				t.Errorf("Done() = %v, want %v", got, c.want)
			}
		})
	}
}
