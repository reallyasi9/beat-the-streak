package bts

import "testing"

func TestString(t *testing.T) {
	p := EmptyPredictions(TeamList{"Apple", "Bananas Tech", "Citrus State University", "Dal", "Extremely Long Named Tech State University", "Fish U"}, 14)
	t.Logf("\n%s", p.String())
}
