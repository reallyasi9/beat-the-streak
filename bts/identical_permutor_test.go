package bts

import (
	"math/big"
	"testing"
)

func check(p1 []int, p2 []int) bool {
	if len(p1) != len(p2) {
		return false
	}
	for i := range p1 {
		if p1[i] != p2[i] {
			return false
		}
	}
	return true
}

func TestIdenticalPermutor(t *testing.T) {
	s := []int{0, 0, 0, 1, 1, 2}
	p := NewIdenticalPermutor(3, 2, 1)

	// Just in case
	if p.Len() != 6 {
		t.Errorf("expected %v, got %v", 6, p.Len())
	}

	// First should be identical
	itr := p.Iterator()
	test := <-itr
	if !check(s, test) {
		t.Fatalf("expected %v, got %v", s, test)
	}

	// Second should not be identical
	test = <-itr
	if check(s, test) {
		t.Fatalf("expected different from %v, got %v", s, test)
	}

	s2 := []int{1, 1, 1, 2}
	p2 := NewIdenticalPermutor(0, 3, 1)

	// Should be only 4 of these
	n := 0
	itr2 := p2.Iterator()
	for test = range itr2 {
		t.Log(test)
		n++
	}

	if n != 4 {
		t.Fatalf("expected 4, got %v", n)
	}

	// Last should be different than first
	if check(s2, test) {
		t.Fatalf("expected different from %v, got %v", s2, test)
	}

	// Should be (a+b)!/a!/b! of these
	p3 := NewIdenticalPermutor(2, 3)
	n = 0
	itr3 := p3.Iterator()
	for test = range itr3 {
		t.Log(test)
		n++
	}

	factest := factorial(2 + 3)
	factest.Div(factest, factorial(2))
	factest.Div(factest, factorial(3))
	if factest.Cmp(big.NewInt(int64(n))) != 0 {
		t.Fatalf("expected 6!/2!/3!, got %v", n)
	}
	nop := p3.NumberOfPermutations()
	if factest.Cmp(nop) != 0 {
		t.Errorf("expected %v, got %v", factest, nop)
	}
}

func BenchmarkIdenticalPermutor10(b *testing.B) {
	p := NewIdenticalPermutor(4, 3, 3)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		itr := p.Iterator()
		for range itr {
			// Count them all!
		}
	}
}
