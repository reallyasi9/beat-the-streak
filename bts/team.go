package bts

import (
	"fmt"
	"strings"
)

// Team play game against Team.
type Team string

// TeamList implements the sort.Interface interface and represents a list of Teams.
type TeamList []Team

// BYE represents a bye week for a team in a schedule.
const BYE = Team("BYE")

// Len calculates the length of the TeamList (implements sort.Interface interface)
func (t TeamList) Len() int {
	return len(t)
}

// Less reports whether (implements sort.Interface interface)
func (t TeamList) Less(i, j int) bool {
	return t[i] < t[j]
}

// Swap swaps the elements with indexes i and j (implements sort.Interface interface)
func (t TeamList) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

// Clone just clones the probabilities.
func (t TeamList) Clone() TeamList {
	out := make(TeamList, t.Len())
	for i, team := range t {
		out[i] = team
	}
	return out
}

// Validate a TeamList against a given Probabilities map.
func (t TeamList) validate(p Predictions) error {
	for _, team := range t {
		if _, ok := p.probs[team]; !ok {
			return fmt.Errorf("team '%s' not in predictions", team)
		}
	}
	return nil
}

func maxSlice(s string, max int) string {
	end := max
	if len(s) < max {
		end = len(s)
	}
	return s[:end]
}

// Shortened returnes a shortened version of the team name for easier display (max 4 characters, all upper case).
func (t Team) Shortened() string {
	split := strings.SplitN(string(t), " ", 4)
	var b strings.Builder
	switch len(split) {
	case 0:
		return "BYE"
	case 1:
		return strings.ToUpper(maxSlice(split[0], 4))
	case 2:
		b.WriteString(strings.ToUpper(maxSlice(split[0], 2)))
		b.WriteString(strings.ToUpper(maxSlice(split[1], 2)))
		return b.String()
	default:
		n := 4
		if len(split) < n {
			n = len(split)
		}
		for i := 0; i < n; i++ {
			b.WriteString(strings.ToUpper(maxSlice(split[i], 1)))
		}
		return b.String()
	}
}
