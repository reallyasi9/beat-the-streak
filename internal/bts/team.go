package bts

// Team play game against Team.
type Team string

// TeamList implements the sort.Interface interface and represents a list of Teams.
type TeamList []Team

// BYE represents a bye week for a team in a schedule.
const BYE = Team("BYE")

// NONE represents a null pick--used when a player uses a pick bye on a week.
const NONE = Team("----")

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

// Clone just clones the probabilities.
func (t TeamList) Clone() TeamList {
	out := make(TeamList, t.Len())
	copy(out, t)
	return out
}
