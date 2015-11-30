package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Convenience type, so I can parse a list of strings from the command line
type selection sort.StringSlice

// StringSlice interface
func (s selection) Len() int           { return len(s) }
func (s selection) Less(i, j int) bool { return s[i] < s[j] }
func (s selection) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// String method, part of the flag.Value interface
func (s *selection) String() string {
	return fmt.Sprint(*s)
}

// Set method, part of the flag.Value interface
func (s *selection) Set(value string) error {
	if len(*s) > 0 {
		return errors.New("selection flag already set")
	}
	for _, sel := range strings.Split(value, ",") {
		*s = append(*s, sel)
	}
	return nil
}

// Filter a single team from the selection
func (s *selection) CopyWithoutTeam(t string) (selection, error) {
	out := make(selection, len(*s))
	copy(out, *s)
	found := -1
	for i, sel := range out {
		if sel == t {
			found = i
			out = append(out[:i], out[i+1:]...)
			break
		}
	}
	if found < 0 {
		return out, fmt.Errorf("team %s not found", t)
	}
	return out, nil
}

type probabilityMap map[string][]float64

func (p *probabilityMap) KeepTeams(s selection) error {
	out := make(probabilityMap)
	for _, sel := range s {
		v, exists := (*p)[sel]
		if !exists {
			return fmt.Errorf("key %s not present in map", sel)
		}
		out[sel] = v
	}
	*p = out
	return nil
}

func (p *probabilityMap) CopyWithoutTeam(t string) (probabilityMap, error) {
	out := make(probabilityMap)
	_, exists := (*p)[t]
	if !exists {
		return out, fmt.Errorf("key %s not present in map", t)
	}
	for k, v := range *p {
		if k == t {
			continue
		}
		out[k] = v
	}
	return out, nil
}

func (p *probabilityMap) FilterWeeks(minWeek int) error {
	for k, v := range *p {
		(*p)[k] = v[minWeek-1:]
	}
	return nil
}

func (p *probabilityMap) TotalProb(s selection) (float64, error) {
	prob := float64(1)
	for i, sel := range s {
		if len((*p)[sel]) != len(s) {
			return 0., fmt.Errorf("length of selection (%d) does not match remaining weeks (%d)", len(s), len((*p)[sel]))
		}
		prob *= (*p)[sel][i]
	}
	return prob, nil
}

// Convenience type, so I can return both a probability and a selection from a goroutine
type orderperm struct {
	prob   float64
	perm   selection
	ddteam string
	ddweek int
}

func (o *orderperm) UpdateGT(other orderperm) bool {
	if other.prob > o.prob {
		o.prob = other.prob
		copy(o.perm, other.perm)
		o.ddteam = other.ddteam
		o.ddweek = other.ddweek
		return true
	}
	return false
}

func (o *orderperm) CSV(pm probabilityMap, nWeeks int) (csv []string) {
	csv = append(csv, fmt.Sprint(o.prob))
	skippedWeeks := nWeeks - len(o.perm)
	for i := 0; i < skippedWeeks; i++ {
		csv = append(csv, fmt.Sprint(i), "", "1")
	}
	for i, t := range o.perm {
		csv = append(csv, fmt.Sprint(i+skippedWeeks), t, fmt.Sprint(pm[t][i]))
	}
	csv = append(csv, fmt.Sprint(o.ddweek), o.ddteam)
	if o.ddweek >= 0 {
		csv = append(csv, fmt.Sprint(pm[o.ddteam][o.ddweek]))
	} else {
		csv = append(csv, "1")
	}
	return csv
}
