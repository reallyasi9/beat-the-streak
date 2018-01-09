package bts

import (
	"bytes"
	"fmt"
	"sort"
)

type Probabilities map[Team][]float64

func (p Probabilities) FilterWeeks(w int) {
	for team, probs := range p {
		p[team] = probs[w-1:]
	}
}

func (p Probabilities) String() string {
	keys := make([]string, len(p))
	i := 0
	for k := range p {
		keys[i] = string(k)
		i++
	}
	sort.Strings(keys)

	nWeeks := 0
	if len(keys) > 0 {
		nWeeks = len(p[Team(keys[0])])
	}

	var buffer bytes.Buffer

	buffer.WriteString("              ")
	for i = 0; i < nWeeks; i++ {
		buffer.WriteString(fmt.Sprintf(" %8d ", i))
	}
	buffer.WriteString("\n")
	for _, k := range keys {
		key := k
		if len(key) > 12 {
			key = k[:12]
		}
		buffer.WriteString(fmt.Sprintf(" %12s ", key))
		for _, v := range p[Team(k)] {
			buffer.WriteString(fmt.Sprintf(" %8.4f ", v))
		}
		buffer.WriteString("\n")
	}

	return buffer.String()
}
