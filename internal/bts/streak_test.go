package bts

import (
	"testing"
)

func TestNewStreak(t *testing.T) {
	rem := Remaining{Team{"A"}, Team{"B"}, Team{"C"}, Team{"D"}}
	ppw := []int{0, 2, 0, 1, 1}
	itr := []int{1, 2, 3, 0}
	s := NewStreak(rem, ppw, itr)

	t.Log(s)
}
