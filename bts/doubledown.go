package bts

type DoubleDown struct {
	Team        Team
	Week        int
	Probability float64
}

func BestWeek(t Team, p Probabilities) *DoubleDown {
	max := 0.
	maxWeek := -1
	for week, prob := range p[t] {
		if prob > max {
			max = prob
			maxWeek = week
		}
	}
	return &DoubleDown{Team: t, Week: maxWeek, Probability: max}
}
