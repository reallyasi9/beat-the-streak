package bts

import (
	"math/big"

	"github.com/segmentio/fasthash/jody"
)

// IndexPermutor permutes an integer range from 0 to N.
type IndexPermutor struct {
	indices []int
}

// IdenticalPermutor permutes a set of potentially repeated integers with n_i copies of integers i=0 to i=N.
type IdenticalPermutor struct {
	indices []int
	sets    []int
}

// NewIndexPermutor creates an IndexPermutor for the integer range [0, size)
func NewIndexPermutor(size int) *IndexPermutor {
	out := make([]int, size)
	for i := 0; i < size; i++ {
		out[i] = i
	}
	return &IndexPermutor{indices: out}
}

// NewIdenticalPermutor creates an IdenticalPermutor for potentially duplicated integers in the range [0, len(setSizes)).
// Each passed ith value in the variatic argument represents the number of identical copies of integer i that is in the set to be permuted.
// For example:
//   NewIdenticalPermutor(2, 0, 3, 1)
// This will create a permutor over the set [0, 0, 2, 2, 2, 3] (2 copies of 0, 0 copies of 1, 3 copies of 2, and 1 copy of 3).
func NewIdenticalPermutor(setSizes ...int) *IdenticalPermutor {
	out := make([]int, 0)
	sets := make([]int, len(setSizes))
	for set, setSize := range setSizes {
		sets[set] = setSize
		for j := 0; j < setSize; j++ {
			out = append(out, set)
		}
	}
	return &IdenticalPermutor{indices: out, sets: sets}
}

// Len returns the length of the set being permuted.
func (ip IndexPermutor) Len() int {
	return len(ip.indices)
}

// Len returns the length of the set being permuted.
func (ip IdenticalPermutor) Len() int {
	return len(ip.indices)
}

func factorial(n int) *big.Int {
	z := new(big.Int)
	return z.MulRange(1, int64(n))
}

// NumberOfPermutations returns the number of permutations possible for the set.
func (ip IndexPermutor) NumberOfPermutations() *big.Int {
	return factorial(ip.Len())
}

// NumberOfPermutations returns the number of permutations possible for the set.
func (ip IdenticalPermutor) NumberOfPermutations() *big.Int {
	fact := factorial(ip.Len())
	for _, set := range ip.sets {
		fact.Div(fact, factorial(set))
	}
	return fact
}

func hash(v []int) uint64 {
	h := jody.HashUint64(uint64(v[0]))
	for _, x := range v[1:] {
		h = jody.AddUint64(h, uint64(x))
	}
	return h
}

func clone(x []int) []int {
	out := make([]int, len(x))
	copy(out, x)
	return out
}

// Iterator returns a channel-backed iterator that produces iterations of the identical sets represented by an IdenticalPermutor.
// The channel closes once all the permutations have been pushed.
// The implementation uses Heap's algorithm (non-recursive) and a map of hashes to keep track of which permutations have been seen already.
// This means for very large permutations, there is some small probability that a hash collision will occur and certain permutations could be skipped in the iteration.
func (ip *IdenticalPermutor) Iterator() <-chan []int {
	ch := make(chan []int, 20)

	go func() {
		visited := make(map[uint64]bool)
		out := make([]int, ip.Len())
		counter := make([]int, len(out))

		copy(out, ip.indices)

		visited[hash(out)] = true
		ch <- clone(out)

		i := 0
		for i < len(out) {
			if counter[i] < i {
				if i%2 == 0 {
					out[0], out[i] = out[i], out[0]
				} else {
					out[counter[i]], out[i] = out[i], out[counter[i]]
				}

				if _, ok := visited[hash(out)]; !ok {
					visited[hash(out)] = true
					ch <- clone(out)
				}

				counter[i]++
				i = 0

			} else {
				counter[i] = 0
				i++
			}
		}
		close(ch)
	}()

	return ch
}

// Iterator returns a channel-backed iterator that produces iterations of the identical sets represented by an IdenticalPermutor.
// The channel closes once all the permutations have been pushed.
// The implementation uses Heap's algorithm (non-recursive).
func (ip *IndexPermutor) Iterator() <-chan []int {
	ch := make(chan []int, 20)

	go func() {
		out := make([]int, ip.Len())
		counter := make([]int, len(out))

		copy(out, ip.indices)

		ch <- clone(out)

		i := 0
		for i < len(out) {
			if counter[i] < i {
				if i%2 == 0 {
					out[0], out[i] = out[i], out[0]
				} else {
					out[counter[i]], out[i] = out[i], out[counter[i]]
				}

				ch <- clone(out)

				counter[i]++
				i = 0

			} else {
				counter[i] = 0
				i++
			}
		}
		close(ch)
	}()

	return ch
}
