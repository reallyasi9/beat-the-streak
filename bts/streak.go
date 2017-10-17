package bts

import (
	"bytes"
	"fmt"
	"strconv"
)

type Streak struct {
	Teams       TeamList
	DD          *DoubleDown
	Probability float64
}

func (s Streak) Clone() Streak {
	t2 := s.Teams.Clone()
	var dd2 *DoubleDown
	if s.DD != nil {
		dd2 = &DoubleDown{Team: s.DD.Team, Probability: s.DD.Probability, Week: s.DD.Week}
	}
	p2 := s.Probability
	return Streak{Teams: t2, DD: dd2, Probability: p2}
}

func (s Streak) String(p Probabilities) string {
	var out bytes.Buffer
	out.WriteString("[")
	lengths := make([]int, len(s.Teams))
	for i, team := range s.Teams {
		out.WriteString(fmt.Sprintf(" %s ", team))
		l := len(team)
		for l <= 7 {
			out.WriteRune(' ')
			l++
		}
		lengths[i] = l
	}
	out.WriteString("]")
	if s.DD != nil {
		out.WriteString(fmt.Sprintf(" %4s @ Week %d", s.DD.Team, s.DD.Week))
	}
	out.WriteString("\n ")
	probs := s.Teams.Probabilities(p)
	for i := range s.Teams {
		l := int64(lengths[i])
		format := "%0." + strconv.FormatInt(l-1, 10) + "f "
		out.WriteString(fmt.Sprintf(format, probs[i]))
	}
	out.WriteRune(' ')
	if s.DD != nil {
		out.WriteString(fmt.Sprintf("%0.3f", s.DD.Probability))
	}
	out.WriteString(fmt.Sprintf(" = %0.3f", s.Probability))
	return out.String()
}

func (s Streak) Permute(c chan<- Streak, p Probabilities) {
	defer close(c)

	teams := s.Teams.Clone()
	dd := s.DD

	tchan := make(chan TeamList)
	go Permute(teams, tchan)
	for t := range tchan {
		teams = t
		prob := teams.Probability(p)
		if dd != nil {
			prob *= dd.Probability
		}
		// first permutation is always free
		c <- Streak{Teams: teams, DD: dd, Probability: prob}
	}

	if dd != nil {
		// permute double down team
		fullTeams := s.Teams.Clone()
		fullTeams = append(fullTeams, dd.Team)

		for i, team := range fullTeams {
			dd2 := BestWeek(team, p)
			teams = fullTeams.Clone()
			teams = append(teams[:i], teams[i+1:]...)

			dd.Team = dd2.Team
			dd.Probability = dd2.Probability
			dd.Week = dd2.Week

			tchan := make(chan TeamList)
			go Permute(teams, tchan)
			for t := range tchan {
				teams = t
				prob := teams.Probability(p)
				if dd != nil {
					prob *= dd.Probability
				}
				// first permutation is always free
				c <- Streak{Teams: teams, DD: dd, Probability: prob}
			}
		}
	}
}

type StreakMap map[string]Streak
