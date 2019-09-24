package bts

import (
	"fmt"
	"strings"
)

// Streak represents a potential streak selection for a contestant.
type Streak struct {
	weeks []TeamList
}

// NewStreak creates a Streak from a list of teams, a selected number of picks per week, and a permutation of the teams into the week.
func NewStreak(teamList Remaining, picksPerWeek []int, indexPermutation []int) *Streak {
	if len(indexPermutation) != len(teamList) {
		panic(fmt.Errorf("number of teams in list %d must be equal to the indices in the permutation %d", len(teamList), len(indexPermutation)))
	}
	picks := make([]TeamList, len(picksPerWeek))
	i := 0
	for week, nPicks := range picksPerWeek {
		picks[week] = make(TeamList, nPicks)
		if nPicks == 0 {
			// No pick--bye used
			picks[week] = append(picks[week], NONE)
			continue
		}
		for p := 0; p < nPicks; p++ {
			if i > len(indexPermutation) {
				panic(fmt.Errorf("sum total of picks per week must not surpass number of teams remaining %d", len(indexPermutation)))
			}
			picks[week][p] = teamList[indexPermutation[i]]
			i++
		}
	}
	return &Streak{weeks: picks}
}

// GetWeek returns the teams selected on a given week.
func (s *Streak) GetWeek(week int) TeamList {
	return s.weeks[week]
}

// FindTeam returns the week in which the given team was selected.
func (s *Streak) FindTeam(team Team) int {
	for week, picks := range s.weeks {
		for _, pick := range picks {
			if pick == team {
				return week
			}
		}
	}
	return -1
}

// NumWeeks returns the number of weeks in the streak.
func (s *Streak) NumWeeks() int {
	return len(s.weeks)
}

func (s *Streak) String() string {
	var out strings.Builder
	for week, tl := range s.weeks {
		out.WriteString(fmt.Sprintf("%2d: ", week))
		for _, t := range tl {
			out.WriteString(fmt.Sprintf("%-4s ", t.Shortened()))
		}
		out.WriteString("\n")
	}
	return out.String()
}
