package bts

import (
	"math/big"
	"testing"
)

func factorial(n int) *big.Int {
	z := new(big.Int)
	return z.MulRange(1, int64(n))
}

func TestIndexPermutor(t *testing.T) {
	ip := MakeIndexPermutor(8)
	if ip.Len() != 8 {
		t.Fatalf("Length of IndexPermutor incorrect: expect 5, got %d\n", ip.Len())
	}

	ip2 := MakeIndexPermutor(8)
	ip.Permute() // zeroth permutation is the same as first permutation
	if ip.Nth().Cmp(bigZero) != 0 {
		t.Fatalf("Permutation number incorrect: expect %v, got %v\n", *bigZero, *(ip.Nth()))
	}

	for i := 0; i < ip.Len(); i++ {
		if ip.Indices[i] != ip2.Indices[i] {
			t.Fatalf("Zeroth permutation incorrect at position %d: expect %v, got %v\n", i, ip2.Indices, ip.Indices)
		}
	}

	ip.Permute() // 1st
	ip.Permute() // 2nd
	ip.Permute() // 3rd
	ip.Permute() // 4th

	ip2.NthPermutation(big.NewInt(4))

	if ip.Nth().Cmp(ip2.Nth()) != 0 {
		t.Fatalf("Permutation number incorrect: expect %v, got %v\n", *(ip2.Nth()), *(ip.Nth()))
	}
	for i := 0; i < ip.Len(); i++ {
		if ip.Indices[i] != ip2.Indices[i] {
			t.Fatalf("Fourth permutation incorrect at position %d: expect %v, got %v\n", i, ip2.Indices, ip.Indices)
		}
	}

	for ip.Permute() {
		// Iterate through all permutations
	}

	fac := factorial(ip.Len())
	fac.Sub(fac, bigOne)
	if ip.Nth().Cmp(fac) != 0 {
		t.Fatalf("Number of permutations incorrect: expect %v, got %v\n", fac, ip.Nth())
	}

	// No more permutations
	ok := ip.Permute()
	if !ok {
		t.Fatalf("More permutations expected, but old permutation returned: %v", *ip)
	}
	if ip.Nth().Cmp(bigZero) != 0 {
		t.Fatalf("Number of permutations incorrect: expect %v, got %v\n", bigZero, ip.Nth())
	}
	ip2.Reset()
	for i := 0; i < ip.Len(); i++ {
		if ip.Indices[i] != ip2.Indices[i] {
			t.Fatalf("Zeroth permutation incorrect at position %d: expect %v, got %v\n", i, ip2.Indices, ip.Indices)
		}
	}

	ip.Permute()          // 0th
	ip.Permute()          // 1st
	ip2 = ip.CloneReset() // -1st (initial state)
	for i := 0; i < ip.Len(); i++ {
		if ip.Indices[i] != ip2.Indices[i] {
			t.Fatalf("Cloned permutation incorrect at position %d: expect %v, got %v\n", i, ip.Indices, ip2.Indices)
		}
	}
	if ip2.Nth().Cmp(ip.Nth()) >= 0 {
		t.Fatalf("Number of permutations incorrect: expected less than %v, got %v\n", ip.Nth(), ip2.Nth())
	}

}

func BenchmarkPermuteOne14(b *testing.B) {
	ip := MakeIndexPermutor(14)
	for i := 0; i < b.N; i++ {
		ip.Permute()
	}
}

func BenchmarkPermuteAll10(b *testing.B) {
	ip := MakeIndexPermutor(10)
	for i := 0; i < b.N; i++ {
		for ip.Permute() {
			// Permute through all possible permutations
		}
	}
}

func BenchmarkPermutatorClone100(b *testing.B) {
	ip := MakeIndexPermutor(100)
	ip.Permute() // just to make things interesting
	for i := 0; i < b.N; i++ {
		ip.CloneReset()
	}
}
