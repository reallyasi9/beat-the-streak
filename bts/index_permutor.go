package bts

import "math/big"

type IndexPermutor struct {
	Indices []int
	counter []int
	nth     *big.Int
	final   bool
}

func (ip *IndexPermutor) Len() int {
	return len(ip.Indices)
}

func (ip *IndexPermutor) Nth() *big.Int {
	n := new(big.Int)
	n.Set(ip.nth).Sub(n, bigOne)
	return n
}

func (ip *IndexPermutor) CloneReset() *IndexPermutor {
	idx := make([]int, ip.Len())
	cnt := make([]int, ip.Len())
	copy(idx, ip.Indices)
	return &IndexPermutor{Indices: idx, counter: cnt, nth: new(big.Int)}
}

func MakeIndexPermutor(n int) *IndexPermutor {
	idx := make([]int, n)
	cnt := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	return &IndexPermutor{Indices: idx, counter: cnt, nth: new(big.Int)}
}

func (ip *IndexPermutor) Reset() {
	for i := range ip.Indices {
		ip.Indices[i] = i
		ip.counter[i] = 0
	}
	ip.nth.Set(bigZero)
	ip.final = false
}

var bigZero *big.Int
var bigOne *big.Int

func init() {
	bigZero = new(big.Int)
	bigOne = big.NewInt(1)
}

func (ip *IndexPermutor) Permute() bool {

	if ip.final {
		ip.Reset()
	}

	// First permutation: self
	if ip.nth.Cmp(bigZero) == 0 {
		ip.nth.Add(ip.nth, bigOne)
		return true
	}

	itr := 0

	for itr < ip.Len() {
		if ip.counter[itr] < itr {
			if itr%2 == 0 {
				ip.Indices[0], ip.Indices[itr] = ip.Indices[itr], ip.Indices[0]
			} else {
				ip.Indices[ip.counter[itr]], ip.Indices[itr] = ip.Indices[itr], ip.Indices[ip.counter[itr]]
			}
			ip.counter[itr]++
			ip.nth.Add(ip.nth, bigOne)
			return true
		}
		ip.counter[itr] = 0
		itr++
	}

	ip.final = true
	return false
}

func (ip *IndexPermutor) NthPermutation(n *big.Int) bool {
	if ip.Nth().Cmp(n) > 0 {
		ip.Reset()
	}

	for ip.Nth().Cmp(n) < 0 {
		if !ip.Permute() {
			return false
		}
	}

	return true
}
