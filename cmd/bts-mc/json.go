package main

import (
	"time"

	"github.com/reallyasi9/beat-the-streak/internal/bts"
)

// Pick is an element of Streak augmented with probability information.
type Pick struct {
	// Selected is the selected winner
	Selected bts.Team
	// Opponent is the opponent chosen to lose to Selected
	Opponent bts.Team
	// Probability is the calculated probability of win
	Probability float64
	// Spread is the calcuated spread
	Spread float64
}

// Week is a week's worth of picks, augmented with total probability information
type Week struct {
	// Picks are the selected teams for the week
	Picks []Pick
	// SeasonWeek is the week of the season this pick represents
	SeasonWeek int
	// Probability is the calculated total probability for the week.
	Probability float64
	// Spread is the calculated total spread for the week.
	Spread float64
}

// StreakOption is one potential streak for a player
type StreakOption struct {
	// FirstSelected is a team selected to win the first week of the steak.
	FirstSelected bts.Team
	// Weeks are the picks for each week in order
	Weeks []Week
	// CumulativeProbability is the calculated cumulative probabilities by week for the streak.
	CumulativeProbability []float64
	// CumulativeSpread is the calculated cumulative spreads by week for the streak.
	CumulativeSpread []float64
	// Probability is the calculated total probability for the streak.
	Probability float64
	// Spread is the calculated total spread for the streak.
	Spread float64
}

// ByProbDesc sorts StreakOptions by probability and spread (descending)
type ByProbDesc []StreakOption

func (a ByProbDesc) Len() int { return len(a) }
func (a ByProbDesc) Less(i, j int) bool {
	if a[i].Probability == a[j].Probability {
		return a[i].Spread > a[j].Spread
	}
	return a[i].Probability > a[j].Probability
}
func (a ByProbDesc) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// PlayerResults is a collection of streak options for a player
type PlayerResults struct {
	// Player is the name of the player
	Player string
	// StartingWeek is the week of the season on which this result starts
	StartingWeek int
	// CalculationStartTime is when the program that produced the results started
	CalculationStartTime time.Time
	// CalculationEndTime is when the results were generated and finalized
	CalculationEndTime time.Time
	// RemainingTeams are the teams remaining to pick
	RemainingTeams []bts.Team
	// RemainingWeekTypes are counts of the types of weeks remaining (byes, singles, double-downs, triple-downs, etc.)
	RemainingWeekTypes []int
	// BestSelection is a list of teams to select next week that gives the best probability of beating the streak
	BestSelection []bts.Team
	// BestProbability is the probability of beating if the best option is taken
	BestProbability float64
	// BestSpread is the total spread if the best option is taken
	BestSpread float64
	// StreakOptions are the best options for each team the player could pick
	StreakOptions []StreakOption
}
