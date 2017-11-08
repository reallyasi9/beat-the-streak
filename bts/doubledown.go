package bts

type DoubleDown struct {
	Team        Team
	Week        int
	Probability float64
	Spread      float64
}

func BestWeek(t Team, p Probabilities, s Spreads) *DoubleDown {
	max := 0.
	maxWeek := -1
	for week, prob := range p[t] {
		if prob > max {
			max = prob
			maxWeek = week
		}
	}
	return &DoubleDown{Team: t, Week: maxWeek, Probability: max, Spread: s[t][maxWeek]}
}
