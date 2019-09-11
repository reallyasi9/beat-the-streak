package bts

import "testing"

func TestDuplicates(t *testing.T) {
	pm := make(PlayerMap)
	pm["A"] = Remaining{"AAA", "BBB", "CCC"}
	pm["B"] = Remaining{"AAA", "BBB", "DDD"}
	pm["C"] = Remaining{"AAA", "BBB", "CCC", "DDD"}
	pm["D"] = Remaining{"AAA", "BBB", "CCC", "DDD"}
	pm["E"] = Remaining{"AAA", "BBB", "DDD", "CCC"}

	pm.Duplicates()
}
