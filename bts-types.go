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

type probabilityMap map[string][]float64

func (p probabilityMap) FilterTeams(s selection) (probabilityMap, error) {
	out := make(probabilityMap)
	for _, sel := range s {
		v, exists := p[sel]
		if exists == false {
			return out, fmt.Errorf("key %s not present in map", sel)
		}
		out[sel] = v
	}
	return out, nil
}

func (p probabilityMap) FilterWeeks(minWeek int) (probabilityMap, error) {
	out := make(probabilityMap)
	for k, v := range p {
		out[k] = v[minWeek-1:]
	}
	return out, nil
}

// Convenience type, so I can return both a probability and a selection from a goroutine
type permutation struct {
	prob   float64
	perm   selection
	ddprob float64
	ddweek int
}
