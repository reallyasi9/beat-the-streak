package bts

import "testing"

func TestString(t *testing.T) {
	p := EmptyPredictions(TeamList{
		Team("Apple"),
		Team("Bananas Tech"),
		Team("Citrus State University"),
		Team("Dal"),
		Team("Extremely Long Named Tech State University"),
		Team("Fish U")}, 14)
	t.Logf("\n%s", p.String())
}
