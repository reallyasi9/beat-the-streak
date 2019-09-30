package bts

import (
	"fmt"
)

// Team play game against Team.
type Team struct {
	Name4 string `firestore:"name_4"`
}

// TeamList implements the sort.Interface interface and represents a list of Teams.
type TeamList []Team

// BYE represents a bye week for a team in a schedule.
var BYE = Team{Name4: "BYE"}

// NONE represents a null pick--used when a player uses a pick bye on a week.
var NONE = Team{Name4: "----"}

// Name gets the team name
func (t Team) Name() string {
	return t.Name4
}

// Len calculates the length of the TeamList (implements sort.Interface interface)
func (t TeamList) Len() int {
	return len(t)
}

// Less reports whether (implements sort.Interface interface)
func (t TeamList) Less(i, j int) bool {
	return t[i].Name() < t[j].Name()
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
