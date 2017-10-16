package bts

import (
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type Player []string

type PlayerMap map[string]Player

func MakePlayers(playerFile string) (PlayerMap, error) {
	playerYaml, err := ioutil.ReadFile(playerFile)
	if err != nil {
		return nil, err
	}

	rm := make(PlayerMap)
	err = yaml.Unmarshal(playerYaml, rm)
	if err != nil {
		return nil, err
	}

	return rm, nil
}

func (pm PlayerMap) InferWeek() (int, error) {
	min := -1
	max := -1
	for name, teams := range pm {
		nteams := len(teams)
		if min == -1 {
			min = nteams
			max = nteams
		} else if nteams > max {
			max = nteams
		} else if nteams < min {
			min = nteams
		}
		if max-min > 1 {
			return -1, fmt.Errorf("player %s does not have a sensible number of teams remaining (%d)", name, nteams)
		}
	}
	return min, nil
}

func (pm PlayerMap) DoubleDownRemaining(week int) (map[string]bool, error) {
	dd := make(map[string]bool)
	for name, teams := range pm {
		nteams := len(teams)
		switch nteams {
		case week:
			dd[name] = false
		case week + 1:
			dd[name] = true
		default:
			return nil, fmt.Errorf("player %s does not have a sensible number of teams remaining (%d)", name, nteams)
		}
	}
	return dd, nil
}

func (pm PlayerMap) Duplicates() map[string][]string {
	out := make(map[string][]string)
	for name1, teams1 := range pm {
		out[name1] = make([]string, 0)
		for name2, teams2 := range pm {
			if _, ok := out[name2]; ok {
				continue // already found you before
			}
			if equal(teams1, teams2) {
				out[name1] = append(out[name1], name2)
			}
		}
	}
	return out
}

func (pm PlayerMap) PlayerNames() []string {
	out := make([]string, len(pm))
	i := 0
	for name := range pm {
		out[i] = name
		i++
	}
	return out
}

func mapSlice(s []string) map[string]bool {
	m := make(map[string]bool)
	for _, st := range s {
		m[st] = true
	}
	return m
}

func equal(s1 []string, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	m1 := mapSlice(s1)
	m2 := mapSlice(s2)
	if len(m1) != len(m2) {
		return false // watch for duplicates!
	}
	for key := range m1 {
		if _, ok := m2[key]; !ok {
			return false
		}
	}
	return true
}
