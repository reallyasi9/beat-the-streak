package bts

import (
	"encoding/json"
	"fmt"
	"math/rand"
)

// Streak represents a potential streak selection for a contestant.
type Streak struct {
	numberOfPicks []int
	teamOrder     TeamList
}

// NewStreak creates a Streak from a list of teams, a selected number of picks per week, and a permutation of the teams into the week.
func NewStreak(teamList Remaining, picksPerWeek []int) *Streak {
	ppw := make([]int, len(picksPerWeek))
	tl := make(Remaining, len(teamList))
	copy(ppw, picksPerWeek)
	copy(tl, teamList)
	return &Streak{numberOfPicks: ppw, teamOrder: TeamList(tl)}
}

// PermuteTeamOrder permutes the order of the streak by the given permutation index.
func (s *Streak) PermuteTeamOrder(indexPermutation []int) {
	newOrder := make(TeamList, s.teamOrder.Len())
	for i, p := range indexPermutation {
		newOrder[i] = s.teamOrder[p]
	}
	s.teamOrder = newOrder
}

// PicksPerWeek sets the number of picks per week. A nil value as an argument returns the old picks per week value with no changes made.
func (s *Streak) PicksPerWeek(ppw []int) []int {
	if ppw == nil {
		return s.numberOfPicks
	}
	copy(s.numberOfPicks, ppw)
	return s.numberOfPicks
}

// GetWeek returns the teams selected on a given week.
func (s *Streak) GetWeek(week int) TeamList {
	if week < 0 || week >= len(s.numberOfPicks) {
		panic(fmt.Errorf("week %d out of range [0,%d]", week, len(s.numberOfPicks)))
	}
	if s.numberOfPicks[week] == 0 {
		return TeamList{NONE} // Bye week
	}
	pick := 0
	for i := 0; i < week; i++ {
		pick += s.numberOfPicks[i]
	}
	return s.teamOrder[pick : pick+s.numberOfPicks[week]]
}

// FindTeam returns the first week in which the given team was selected.
func (s *Streak) FindTeam(team Team) int {

	if team == NONE {
		for week, n := range s.numberOfPicks {
			if n == 0 {
				return week
			}
		}
		return -1
	}

	var i int // index in team order
	for i = 0; i < s.teamOrder.Len(); i++ {
		if s.teamOrder[i] == team {
			break
		}
	}
	if i == s.teamOrder.Len() {
		return -1
	}
	n := 0
	for week, picks := range s.numberOfPicks {
		if picks == 0 {
			// count bye as a pick for that week
			n++
		} else {
			n += picks
		}
		if i < n {
			return week
		}
	}
	return -1
}

// NumWeeks returns the number of weeks in the streak.
func (s *Streak) NumWeeks() int {
	return len(s.numberOfPicks)
}

func (s *Streak) String() string {
	pick := 0

	pickMap := make([]TeamList, s.NumWeeks())

	for week, ppw := range s.numberOfPicks {
		if ppw == 0 {
			pickMap[week] = TeamList{NONE}
			continue
		}
		picks := make(TeamList, ppw)
		for i := 0; i < ppw; i++ {
			picks[i] = s.teamOrder[pick]
			pick++
		}
		pickMap[week] = picks
	}

	b, _ := json.Marshal(pickMap)
	return string(b)
}

// Perturbate randomly swaps two teams in the team order.
// If `picksPerWeekAlso` is `true`, will also randomly swap the picks per week for two weeks.
// This function is not guaranteed to produce a new distinct streak.
func (s *Streak) Perturbate(src rand.Source, picksPerWeekAlso bool) {
	if src == nil {
		src = rand.NewSource(rand.Int63())
	}
	rng := rand.New(src)

	a := rng.Intn(s.teamOrder.Len())
	b := rng.Intn(s.teamOrder.Len())
	s.teamOrder.Swap(a, b)

	if picksPerWeekAlso {
		a = rng.Intn(len(s.numberOfPicks))
		b = rng.Intn(len(s.numberOfPicks))
		s.numberOfPicks[a], s.numberOfPicks[b] = s.numberOfPicks[b], s.numberOfPicks[a]
	}
}

// Clone clones the streak. This results in a new struct with all of the internal objects cloned.
func (s *Streak) Clone() *Streak {
	ppw := make([]int, len(s.numberOfPicks))
	to := make(TeamList, s.teamOrder.Len())
	copy(ppw, s.numberOfPicks)
	copy(to, s.teamOrder)
	return &Streak{numberOfPicks: ppw, teamOrder: to}
}
