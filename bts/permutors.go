package bts

import (
	"github.com/segmentio/fasthash/jody"
)

type IndexPermutor []int
type IdenticalPermutor []int

func NewIndexPermutor(size int) *IndexPermutor {
	out := make(IndexPermutor, size)
	for i := 0; i < size; i++ {
		out[i] = i
	}
	return &out
}

func NewIdenticalPermutor(setSizes ...int) *IdenticalPermutor {
	out := make(IdenticalPermutor, 0)
	for set, setSize := range setSizes {
		for j := 0; j < setSize; j++ {
			out = append(out, set)
		}
	}
	return &out
}

func (ip IndexPermutor) Len() int {
	return len(ip)
}

func (ip IdenticalPermutor) Len() int {
	return len(ip)
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

func (ip *IdenticalPermutor) Iterator() <-chan []int {
	ch := make(chan []int)

	go func() {
		visited := make(map[uint64]bool)
		out := make([]int, len(*ip))
		counter := make([]int, len(out))

		copy(out, *ip)

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

func (ip *IndexPermutor) Iterator() <-chan []int {
	ch := make(chan []int)

	go func() {
		out := make([]int, len(*ip))
		counter := make([]int, len(out))

		copy(out, *ip)

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
