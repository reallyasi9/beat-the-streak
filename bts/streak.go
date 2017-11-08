package bts

import (
	"bytes"
	"fmt"
)

type Streak struct {
	Teams       TeamList
	DD          *DoubleDown
	Probability float64
	Spreads     []float64
}

func (s Streak) Clone() Streak {
	t2 := s.Teams.Clone()
	var dd2 *DoubleDown
	if s.DD != nil {
		dd2 = &DoubleDown{Team: s.DD.Team, Probability: s.DD.Probability, Week: s.DD.Week}
	}
	p2 := s.Probability
	s2 := make([]float64, len(s.Spreads))
	copy(s2, s.Spreads)
	return Streak{Teams: t2, DD: dd2, Probability: p2, Spreads: s2}
}

func (s Streak) String(p Probabilities) string {
	var out bytes.Buffer
	out.WriteString("[")

	// Discover max team name length
	maxLen := 0
	for _, team := range s.Teams {
		if len(team) > maxLen {
			maxLen = len(team)
		}
	}

	// Lookup "*" in fmt documentation if you don't believe that this will work
	for _, team := range s.Teams {
		out.WriteString(fmt.Sprintf(" %-[1]*[2]s ", maxLen, team))
	}
	out.WriteString("]")

	if s.DD != nil {
		out.WriteString(fmt.Sprintf(" %-[1]*[2]s @ Week %2[3]d", maxLen, s.DD.Team, s.DD.Week))
	}
	out.WriteString("\n ")

	probs := s.Teams.Probabilities(p)
	sumSpread := 0.
	for i := range s.Teams {
		out.WriteString(fmt.Sprintf(" %-[1]*.4[2]f/%-5.1[3]f ", maxLen-6, probs[i], s.Spreads[i]))
		sumSpread += s.Spreads[i]
	}
	out.WriteRune(' ')
	if s.DD != nil {
		out.WriteString(fmt.Sprintf("%-[1]*.4[2]f/%-5.1[3]f", maxLen-6, s.DD.Probability, s.DD.Spread))
		sumSpread += s.DD.Spread
	}
	out.WriteString(fmt.Sprintf(" = %-[1]*.4[2]g/%-5.1[3]f", maxLen-6, s.Probability, sumSpread))
	return out.String()
}

func (s Streak) Permute(c chan<- Streak, p Probabilities, spreads Spreads) {
	defer close(c)

	teams := s.Teams.Clone()
	dd := s.DD

	tchan := make(chan TeamList, 100)
	go Permute(teams, tchan)
	for t := range tchan {
		teams = t
		prob := teams.Probability(p)
		if dd != nil {
			prob *= dd.Probability
		}
		spr := teams.Spreads(spreads)
		// first permutation is always free
		c <- Streak{Teams: teams, DD: dd, Probability: prob, Spreads: spr}
	}

	if dd != nil {
		// permute double down team
		fullTeams := s.Teams.Clone()
		fullTeams = append(fullTeams, dd.Team)

		for i, team := range fullTeams {
			dd2 := BestWeek(team, p, spreads)
			teams = fullTeams.Clone()
			teams = append(teams[:i], teams[i+1:]...)

			dd.Team = dd2.Team
			dd.Probability = dd2.Probability
			dd.Week = dd2.Week
			dd.Spread = dd2.Spread

			tchan := make(chan TeamList, 100)
			go Permute(teams, tchan)
			for t := range tchan {
				teams = t
				prob := teams.Probability(p)
				if dd != nil {
					prob *= dd.Probability
				}
				spr := teams.Spreads(spreads)
				// first permutation is always free
				c <- Streak{Teams: teams, DD: dd, Probability: prob, Spreads: spr}
			}
		}
	}
}

// StreakMap is a simple map of player names to streaks
type StreakMap map[string]Streak

// StreakByProb implements sort.Interface for Streaks, sorting by probabilitiy (ascending)
type StreakByProb []Streak

func (s StreakByProb) Len() int           { return len(s) }
func (s StreakByProb) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s StreakByProb) Less(i, j int) bool { return s[i].Probability > s[j].Probability }
