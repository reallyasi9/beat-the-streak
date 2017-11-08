package bts

type Probabilities map[Team][]float64

func (p Probabilities) FilterWeeks(w int) {
	for team, probs := range p {
		p[team] = probs[w-1:]
	}
}
