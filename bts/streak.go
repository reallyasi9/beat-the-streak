package bts

import (
	"bytes"
	"fmt"
)

type Streak struct {
	Teams TeamList
	DD    *DoubleDown
}

func (s *Streak) Clone() Streak {
	t2 := s.Teams.Clone()
	var dd2 *DoubleDown
	if s.DD != nil {
		dd2 = &DoubleDown{Team: s.DD.Team, Week: s.DD.Week}
	}
	return Streak{Teams: t2, DD: dd2}
}

func (s *Streak) String(p Probabilities, spr Spreads) string {
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
	spreads := s.Teams.Spreads(spr)
	sumSpread := 0.
	totProb := 1.
	for i := range s.Teams {
		out.WriteString(fmt.Sprintf(" %-[1]*.4[2]f/%-5.1[3]f ", maxLen-6, probs[i], spreads[i]))
		sumSpread += spreads[i]
		totProb *= probs[i]
	}
	out.WriteRune(' ')
	if s.DD != nil {
		prob := p[s.DD.Team][s.DD.Week]
		spread := spr[s.DD.Team][s.DD.Week]
		out.WriteString(fmt.Sprintf("%-[1]*.4[2]f/%-5.1[3]f", maxLen-6, prob, spread))
		sumSpread += spread
		totProb *= prob
	}
	out.WriteString(fmt.Sprintf(" = %-[1]*.4[2]g/%-5.1[3]f", maxLen-6, totProb, sumSpread))
	return out.String()
}

func (s Streak) Permute(c chan<- StreakProb, p Probabilities) {
	defer close(c)

	// Results channel
	tchan := make(chan TeamList, 100)
	Permute(s.Teams, tchan)

	for t := range tchan {
		c <- StreakProbability(&Streak{Teams: t, DD: s.DD}, p)
	}

}

// StreakMap is a simple map of player names to streaks
type StreakMap map[string]Streak

type StreakProb struct {
	Streak *Streak
	Prob   float64
}

func StreakProbability(s *Streak, p Probabilities) StreakProb {
	prob := s.Teams.Probability(p)
	if s.DD != nil {
		prob *= p[s.DD.Team][s.DD.Week]
	}
	return StreakProb{Streak: s, Prob: prob}
}

type StreaksByProb []StreakProb

func (s StreaksByProb) Len() int           { return len(s) }
func (s StreaksByProb) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s StreaksByProb) Less(i, j int) bool { return s[i].Prob > s[j].Prob }
