package bts

import (
	"io/ioutil"
	"net/http"
)

func getURLBody(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// TeamPermute creates all possible permutations of a sort.Interface and issues them
// to a chan.  This uses the non-recursive form of Heap's algorithm, which is
// well-suited for a goroutine.  See https://en.wikipedia.org/wiki/Heap%27s_algorithm
func TeamPermute(s TeamList, c chan<- TeamList) {
	defer close(c)

	// First permutation: self
	out := s.Clone()
	c <- out.Clone()

	count := make([]int, out.Len())

	i := 0
	for i < out.Len() {
		if count[i] < i {
			if i%2 == 0 {
				out.Swap(0, i)
			} else {
				out.Swap(count[i], i)
			}
			c <- out.Clone()
			count[i]++
			i = 0
		} else {
			count[i] = 0
			i++
		}
	}
}

func MapSlice(s []string) map[string]bool {
	m := make(map[string]bool)
	for _, st := range s {
		m[st] = true
	}
	return m
}

func SliceMap(m map[string]bool) []string {
	a := make([]string, 0)
	for k, v := range m {
		if v {
			a = append(a, k)
		}
	}
	return a
}
