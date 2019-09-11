package bts

import (
	"io/ioutil"
	"sort"

	"github.com/segmentio/fasthash/jody"

	yaml "gopkg.in/yaml.v2"
)

// Remaining represents a player's teams remaining.
type Remaining TeamList

// PlayerMap associates a player's name to the teams remaining.
type PlayerMap map[string]Remaining

// MakePlayers parses a YAML file and produces a map of remaining players.
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

// Duplicates returns a list of Players who are duplicates of one another.
func (pm PlayerMap) Duplicates() map[string][]string {

	playerHashes := make(map[uint64][]string)
	for name, remaining := range pm {
		hash := jody.HashString64("")
		sort.Sort(TeamList(remaining))
		for _, team := range remaining {
			hash = jody.AddString64(hash, string(team))
		}
		if _, ok := playerHashes[hash]; ok {
			playerHashes[hash] = append(playerHashes[hash], name)
		} else {
			playerHashes[hash] = []string{name}
		}
	}

	out := make(map[string][]string)
	for _, duplicates := range playerHashes {
		out[duplicates[0]] = make([]string, len(duplicates))
		for i, dup := range duplicates {
			out[duplicates[0]][i] = dup
		}
	}

	return out
}

// PlayerNames returns the unique names of the players in the PlayerMap.
func (pm PlayerMap) PlayerNames() []string {
	out := make([]string, len(pm))
	i := 0
	for name := range pm {
		out[i] = name
		i++
	}
	return out
}
