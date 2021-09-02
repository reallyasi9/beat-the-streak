package bts

import (
	"fmt"
	"sort"
	"strings"
)

// Predictions represents the probability of win and predicted spread for every team for each week.
type Predictions struct {
	probs   map[Team][]float64
	spreads map[Team][]float64
}

// EmptyPredictions returns empty probabilities for a set of teams and a given number of weeks.
func EmptyPredictions(teamList TeamList, nWeeks int) *Predictions {
	p := make(map[Team][]float64)
	s := make(map[Team][]float64)
	for _, team := range teamList {
		p[team] = make([]float64, nWeeks)
		s[team] = make([]float64, nWeeks)
	}
	return &Predictions{probs: p, spreads: s}
}

// FilterWeeks filters the Predictions by removing weeks prior to the given one.
func (p *Predictions) FilterWeeks(w int) {
	if w <= 0 {
		return
	}
	for team := range p.probs {
		p.probs[team] = p.probs[team][w:]
		p.spreads[team] = p.spreads[team][w:]
	}
}

// GetProbability returns the probability that the given team wins in the given week.
// Bye weeks have a probability of 1.
func (p *Predictions) GetProbability(team Team, week int) float64 {
	if team == NONE {
		return 1.
	}
	if team == BYE {
		return 0.
	}
	return p.probs[team][week]
}

// GetSpread returns the predicted spread for a given team in a given week.
// Teams on bye weeks are given a spread of 0.
func (p *Predictions) GetSpread(team Team, week int) float64 {
	if team == NONE || team == BYE {
		return 0.
	}
	return p.spreads[team][week]
}

// AccumulateStreak returns the accumulated probability of win a total spread for each week of the given streak.
func AccumulateStreak(p *Predictions, s *Streak) (cumprobs, cumspreads []float64) {
	cumprobs = make([]float64, s.NumWeeks())
	cumspreads = make([]float64, s.NumWeeks())

	cp := 1.
	cs := 0.

	for week := 0; week < s.NumWeeks(); week++ {
		picks := s.GetWeek(week)
		for _, pick := range picks {
			cp *= p.GetProbability(pick, week)
			cs += p.GetSpread(pick, week)
		}
		cumprobs[week] = cp
		cumspreads[week] = cs
	}

	return
}

// SummarizeStreak returns the total predicted probability of beating a streak and the total spread.
func SummarizeStreak(p *Predictions, s *Streak) (prob, spread float64) {
	prob = 1.
	spread = 0.

	for week := 0; week < s.NumWeeks(); week++ {
		picks := s.GetWeek(week)
		for _, pick := range picks {
			prob *= p.GetProbability(pick, week)
			spread += p.GetSpread(pick, week)
		}
	}

	return
}

// MakePredictions uses a schedule and a model to build a map of predictions for fast lookup.
func MakePredictions(s *Schedule, m PredictionModel) *Predictions {
	tl := s.TeamList()
	nWeeks := s.NumWeeks()

	probs := make(map[Team][]float64)
	spreads := make(map[Team][]float64)

	for _, t1 := range tl {
		probs[t1] = make([]float64, nWeeks)
		spreads[t1] = make([]float64, nWeeks)
		for week := 0; week < nWeeks; week++ {
			probs[t1][week], spreads[t1][week] = m.Predict(s.Get(t1, week))
		}
	}

	return &Predictions{probs: probs, spreads: spreads}
}

func (p Predictions) String() string {
	keys := make(TeamList, len(p.probs))
	i := 0
	for k := range p.probs {
		keys[i] = k
		i++
	}
	sort.Sort(keys)

	nWeeks := 0
	for _, q := range p.probs {
		nWeeks = len(q)
		break
	}

	var buffer strings.Builder

	buffer.WriteString("     ")
	for i = 0; i < nWeeks; i++ {
		buffer.WriteString(fmt.Sprintf(" %-13d ", i))
	}
	buffer.WriteString("\n")
	for _, k := range keys {
		buffer.WriteString(fmt.Sprintf("%-4s ", k))
		for w, v := range p.probs[k] {
			buffer.WriteString(fmt.Sprintf(" %5.3f(%+6.2f) ", v, p.spreads[k][w]))
		}
		buffer.WriteString("\n")
	}

	return buffer.String()
}
