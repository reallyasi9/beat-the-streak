package bts

type Team string

// TeamList implements the sort.Interface interface and represents a list of Teams.
type TeamList []Team

// Len calculates the length of the TeamList (implements sort.Interface interface)
func (t TeamList) Len() int {
	return len(t)
}

// Less reports whether (implements sort.Interface interface)
func (t TeamList) Less(i, j int) bool {
	return t[i] < t[j]
}

// Swap swaps the elements with indexes i and j (implements sort.Interface interface)
func (t TeamList) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t TeamList) Clone() TeamList {
	out := make(TeamList, t.Len())
	for i, team := range t {
		out[i] = team
	}
	return out
}

// Probabilities returns the probabilities of the team list for the given order
func (t TeamList) Probabilities(p Probabilities) []float64 {
	out := make([]float64, t.Len())
	for i, team := range t {
		out[i] = p[team][i]
	}
	return out
}

// Probability just multiplies out probabilities
func (t TeamList) Probability(p Probabilities) float64 {
	prob := 1.
	for i, team := range t {
		prob *= p[team][i]
	}
	return prob
}

// Spreads returns the predicted spreads for each game
func (t TeamList) Spreads(s Spreads) []float64 {
	out := make([]float64, t.Len())
	for i, team := range t {
		out[i] = s[team][i]
	}
	return out
}
