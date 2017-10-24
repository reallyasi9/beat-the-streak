package bts

import (
	"fmt"
	"io/ioutil"
	"sort"

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
	return 14 - min, nil
}

func (pm PlayerMap) DoubleDownRemaining(week int) (map[string]bool, error) {
	dd := make(map[string]bool)
	for name, teams := range pm {
		nteams := len(teams)
		switch nteams {
		case 14 - week:
			dd[name] = false
		case 15 - week:
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
				delete(pm, name2)
			}
		}
		if len(out[name1]) == 0 {
			delete(out, name1)
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

func (p Player) BestStreak(probs Probabilities, spreads Spreads, doubleDown bool, topn int) StreakByProb {

	// Convert player (a list of team names) to a TeamList
	teams := make(TeamList, len(p))
	for i, t := range p {
		teams[i] = Team(t)
	}

	var ddTeam *DoubleDown
	// If double down still avaialbe, start by making the first team the DD and
	// cut down the number of teams in the list by one.
	if doubleDown {
		ddTeam = BestWeek(teams[0], probs, spreads)
		teams = teams[1:]
	}

	// Create output
	byProb := make(StreakByProb, topn)
	// bySpread := make(StreakBySpread, topn)

	// Create a first streak
	streak := Streak{
		Teams:       teams,
		DD:          ddTeam,
		Probability: teams.Probability(probs),
		Spreads:     teams.Spreads(spreads),
	}

	// Channel to accept permutaitons
	results := make(chan Streak, 100) // large-ish buffer
	// Permute the streak.
	go streak.Permute(results, probs, spreads)

	// Read from the channel to see which streak is best
	for result := range results {
		if result.Probability > byProb[topn-1].Probability {
			byProb = append(byProb, result.Clone())
			sort.Sort(byProb)
			byProb = byProb[:topn]
		}
		// if result.Spread < bySpread[0].Spread {
		// 	bySpread = append(bySpread, result.Clone())
		// 	sort.Sort(bySpread)
		// 	bySpread = bySpread[:topn]
		// }
	}

	return byProb //, bySpread
}

func equal(s1 []string, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	m1 := MapSlice(s1)
	m2 := MapSlice(s2)
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