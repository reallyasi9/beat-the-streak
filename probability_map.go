package main

import "fmt"

type probabilityMap map[string][]float64

func (p *probabilityMap) CopyWithTeams(s selection) (probabilityMap, error) {
	out := make(probabilityMap)
	for _, sel := range s {
		v, exists := (*p)[sel]
		if !exists {
			return out, fmt.Errorf("key %s not present in map", sel)
		}
		out[sel] = v
	}
	return out, nil
}

func (p *probabilityMap) CopyWithoutTeam(t string) (probabilityMap, error) {
	out := make(probabilityMap)
	_, exists := (*p)[t]
	if !exists {
		return out, fmt.Errorf("key %s not present in map", t)
	}
	for k, v := range *p {
		if k == t {
			continue
		}
		out[k] = v
	}
	return out, nil
}

func (p *probabilityMap) FilterWeeks(minWeek int) error {
	for k, v := range *p {
		(*p)[k] = v[minWeek-1:]
	}
	return nil
}

func (p *probabilityMap) TotalProb(s selection) (float64, error) {
	prob := float64(1)
	for i, sel := range s {
		// if len((*p)[sel]) != len(s) {
		// 	return 0., fmt.Errorf("length of selection (%d) does not match remaining weeks (%d)", len(s), len((*p)[sel]))
		// }
		prob *= (*p)[sel][i]
	}
	return prob, nil
}

func (p *probabilityMap) PrintProbs(o orderPerm) {
	probs := make([]float64, len(o.perm))
	fmt.Print("[")
	for i, sel := range o.perm {
		fmt.Printf(" %4s ", sel)
		probs[i] = (*p)[sel][i]
	}
	fmt.Print("]")
	if o.ddweek >= 0 {
		fmt.Printf(" %4s @ %d", o.ddteam, o.ddweek+1)
	}
	fmt.Print("\n ")
	for _, pr := range probs {
		fmt.Printf("%0.3f ", pr)
	}
	fmt.Print(" ")
	if o.ddweek >= 0 {
		fmt.Printf("%0.3f", (*p)[o.ddteam][o.ddweek])
	} else {
		fmt.Print("     ")
	}
	fmt.Printf(" = %f\n", o.prob)
}

func (p *probabilityMap) Teams() []string {
	keys := make([]string, len(*p))

	i := 0
	for k := range *p {
		keys[i] = k
		i++
	}

	return keys
}
