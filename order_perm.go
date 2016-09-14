package main

import "fmt"

// Convenience type, so I can return both a probability and a selection from a goroutine
type orderPerm struct {
	prob   float64
	perm   selection
	ddteam string
	ddweek int
}

func (o orderPerm) String() string {
	if o.ddweek > 0 {
		return fmt.Sprintf("%s %s @ %d Prob %f", o.perm, o.ddteam, o.ddweek+1, o.prob)
	}
	return fmt.Sprintf("%s Prob %f", o.perm, o.prob)
}

func (o *orderPerm) UpdateGT(other orderPerm) bool {
	if other.prob > o.prob {
		o.prob = other.prob
		o.perm = make(selection, len(other.perm))
		copy(o.perm, other.perm)
		o.ddteam = other.ddteam
		o.ddweek = other.ddweek
		return true
	}
	return false
}
