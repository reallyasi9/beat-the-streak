package bts

import (
	"testing"
)

func TestNewStreak(t *testing.T) {
	rem := Remaining{"A", "B", "C", "D"}
	ppw := []int{0, 2, 0, 1, 1}
	itr := []int{1, 2, 3, 0}
	s := NewStreak(rem, ppw, itr)

	t.Log(s)
}
