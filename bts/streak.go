package bts

import (
	"bytes"
	"fmt"
	"sort"
)

type Streak struct {
	Teams       TeamList
	DD          *DoubleDown
	Probability float64
}

func (s Streak) String() string {
	var out bytes.Buffer
	out.WriteString("[")
	for _, team := range s.Teams {
		out.WriteString(fmt.Sprintf(" %4s ", team.Name))
	}
	out.WriteString("]")
	if s.DD != nil {
		out.WriteString(fmt.Sprintf(" %4s @ Week %d", s.DD.Team, s.DD.Week))
	}
	out.WriteString("\n ")
	for _, team := range s.Teams {
		out.WriteString(fmt.Sprintf("%0.3f ", team.Probability))
	}
	out.WriteString(" ")
	if s.DD != nil {
		out.WriteString(fmt.Sprintf("%0.3f", s.DD.Probability))
	} else {
		out.WriteString("     ")
	}
	out.WriteString(fmt.Sprintf(" = %0.3f", s.Probability))
	return out.String()
}

func (s Streak) Permute(c chan<- Streak, p Probabilities) {
	teams := s.Teams.Clone()
	dd := s.DD
	prob := s.Probability
	// first permutation is always free
	tchan := make(chan sort.Interface)
	Permute(teams, tchan)
	for teams := range tchan {
		s.Teams = (TeamList) teams
		s.Probability = teams.Probability()
		if s.DD != nil {
			s.Probability *= s.DD.Probability
		}
		c <- s
	}

	if s.DD != nil {
		// permute double down team

	}
}

type StreakMap map[string]Streak
