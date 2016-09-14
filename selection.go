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
