package main

import (
	"fmt"
	"strings"
	"time"
)

// packet is one voice packet as heard: its frame number (10 ms units) and
// terminator flag.
type packet struct {
	frame      uint64
	terminator bool
	at         time.Time
}

// spurt is one talk spurt and what is wrong with it.
type spurt struct {
	first, last     uint64
	packets         int
	terminators     int
	gaps, repeats   int  // frame number steps above 2, or not upward
	afterTerminator bool // began right after a terminator (an extra packet)
	open            bool // still running at the end (no terminator yet)
}

type report struct{ spurts []spurt }

const (
	frameStep  = 2 // 20 ms packets
	spurtPause = 300 * time.Millisecond
	extraGap   = 100 * time.Millisecond
)

// check splits one speaker's packets into spurts: a terminator or a pause
// ends one. A last spurt without a terminator that was still running at
// end is open, not wrong.
func check(ps []packet, end time.Time) report {
	var r report
	var cur *spurt
	var prev packet
	for i, p := range ps {
		if cur == nil || prev.terminator || p.at.Sub(prev.at) > spurtPause {
			r.spurts = append(r.spurts, spurt{first: p.frame, afterTerminator: i > 0 && prev.terminator && p.at.Sub(prev.at) < extraGap})
			cur = &r.spurts[len(r.spurts)-1]
		} else if d := int64(p.frame) - int64(prev.frame); d <= 0 {
			cur.repeats++
		} else if d > frameStep {
			cur.gaps++
		}
		cur.packets++
		cur.last = p.frame
		if p.terminator {
			cur.terminators++
		}
		prev = p
	}
	if cur != nil && cur.terminators == 0 && end.Sub(prev.at) <= spurtPause {
		cur.open = true
	}
	return r
}

func (s spurt) ok() bool {
	return (s.terminators == 1 || s.open && s.terminators == 0) && s.gaps == 0 && s.repeats == 0 && !s.afterTerminator
}

func (r report) ok() bool {
	for _, s := range r.spurts {
		if !s.ok() {
			return false
		}
	}
	return len(r.spurts) > 0
}

func (s spurt) String() string {
	var bad []string
	if s.open {
		bad = append(bad, "still running")
	} else if s.terminators != 1 {
		bad = append(bad, fmt.Sprintf("%d terminators", s.terminators))
	}
	if s.gaps > 0 {
		bad = append(bad, fmt.Sprintf("%d gaps", s.gaps))
	}
	if s.repeats > 0 {
		bad = append(bad, fmt.Sprintf("%d repeats", s.repeats))
	}
	if s.afterTerminator {
		bad = append(bad, "extra packet after the terminator")
	}
	status := "ok"
	if s.open && len(bad) == 1 {
		status = "ok, still running"
	} else if len(bad) > 0 {
		status = strings.Join(bad, ", ")
	}
	return fmt.Sprintf("frames %d-%d, %d packets: %s", s.first, s.last, s.packets, status)
}
