package bts

type Team struct {
	Name        string
	Probability float64
}

// TeamList implements the sort.Interface interface and represents a list of Teams.
type TeamList []Team

// Len calculates the length of the TeamList (implements sort.Interface interface)
func (t TeamList) Len() int {
	return len(t)
}

// Less reports whether (implements sort.Interface interface)
func (t TeamList) Less(i, j int) bool {
	return t[i].Name < t[j].Name
}

// Swap swaps the elements with indexes i and j (implements sort.Interface interface)
func (t TeamList) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t TeamList) Clone() TeamList {
	out := make(TeamList, len(t))
	for i, team := range t {
		out[i] = team
	}
	return out
}

// Probability just multiplies out probabilities
func (t TeamList) Probability() float64 {
	p := 1.
	for _, team := range t {
		p *= team.Probability
	}
	return p
}
