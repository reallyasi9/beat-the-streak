package main

import (
	"errors"
	"fmt"
	"strings"
)

// Convenience type, so I can parse a list of strings from the command line
type selection []string

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
