package bts

import "testing"

func TestDuplicates(t *testing.T) {
	pm := make(PlayerMap)
	pm["A"] = &Player{name: "A", remaining: Remaining{Team("AAA"), Team("BBB"), Team("CCC")}, weekTypes: NewIdenticalPermutor(0, 3, 0)}
	pm["B"] = &Player{name: "Dup A Identical", remaining: Remaining{Team("AAA"), Team("BBB"), Team("CCC")}, weekTypes: NewIdenticalPermutor(0, 3, 0)}
	pm["C"] = &Player{name: "Dup A New Order", remaining: Remaining{Team("AAA"), Team("CCC"), Team("BBB")}, weekTypes: NewIdenticalPermutor(0, 3, 0)}
	pm["D"] = &Player{name: "Not A New Teams", remaining: Remaining{Team("AAA"), Team("BBB"), Team("CCC"), Team("DDD")}, weekTypes: NewIdenticalPermutor(0, 3, 0)}
	pm["E"] = &Player{name: "Not A New Weeks", remaining: Remaining{Team("AAA"), Team("BBB"), Team("CCC")}, weekTypes: NewIdenticalPermutor(0, 1, 1)}

	pm.Duplicates()
}
