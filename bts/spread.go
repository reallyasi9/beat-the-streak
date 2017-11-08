package bts

type Spreads map[Team][]float64

func (s Spreads) FilterWeeks(w int) {
	for team, spreads := range s {
		s[team] = spreads[w-1:]
	}
}
