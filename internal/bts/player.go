package bts

import (
	"fmt"
	"io/ioutil"
	"sort"

	"github.com/segmentio/fasthash/jody"

	yaml "gopkg.in/yaml.v2"
)

// Player represents a player's current status in the competition.
type Player struct {
	name      string
	remaining Remaining
	weekTypes *IdenticalPermutor
}

// Name returns the player's name
func (p Player) Name() string {
	return p.name
}

// RemainingTeams returns the list of remaining teams.
func (p Player) RemainingTeams() Remaining {
	return p.remaining
}

// RemainingWeekTypes returns the list of remaining week types.
func (p Player) RemainingWeekTypes() []int {
	return p.weekTypes.sets
}

// RemainingWeeks calculates the number of weeks of picks this player has remaining.
func (p Player) RemainingWeeks() int {
	nWeeks := 0
	for _, ntype := range p.RemainingWeekTypes() {
		nWeeks += ntype
	}
	return nWeeks
}

// RemainingIterator returns an iterator over remaining team indices.
func (p Player) RemainingIterator() <-chan []int {
	return NewIndexPermutor(len(p.remaining)).Iterator()
}

// WeekTypeIterator returns an iterator over remaining week types.
func (p Player) WeekTypeIterator() <-chan []int {
	return p.weekTypes.Iterator()
}

// Remaining represents a player's teams remaining.
type Remaining TeamList

// RemainingMap associates a player's name to the teams remaining.
type RemainingMap map[string]Remaining

// WeeksMap associates a player's name to the remaining weeks.
type WeeksMap map[string][]int

// PlayerMap associates a player's name with a status.
type PlayerMap map[string]*Player

// NewPlayer builds a new player
func NewPlayer(name string, remaining Remaining, weekTypesRemaining []int) (*Player, error) {
	nTeams := len(remaining)
	nPicks := 0
	for itype, ntype := range weekTypesRemaining {
		nPicks += itype * ntype
	}
	if nPicks != nTeams {
		return nil, fmt.Errorf("number of teams remaining (%d) must equal number of picks remaining (%d)", nTeams, nPicks)
	}
	return &Player{
		name:      name,
		remaining: remaining,
		weekTypes: NewIdenticalPermutor(weekTypesRemaining...),
	}, nil
}

// MakePlayers parses a YAML file and produces a map of remaining players.
func MakePlayers(playerFile string, weekTypeFile string) (PlayerMap, error) {
	playerYaml, err := ioutil.ReadFile(playerFile)
	if err != nil {
		return nil, err
	}

	rm := make(RemainingMap)
	err = yaml.Unmarshal(playerYaml, rm)
	if err != nil {
		return nil, err
	}

	// Figure out week types if necessary
	wm := make(WeeksMap)
	if weekTypeFile == "" {
		nRemaining := -1
		for name, remaining := range rm {
			if nRemaining > 0 && len(remaining) != nRemaining {
				return nil, fmt.Errorf("number of remaining teams inconsistent: must specify a weeks-remaining file")
			}
			nRemaining = len(remaining)
			wm[name] = []int{0, nRemaining}
		}
	} else {
		weeksYaml, err := ioutil.ReadFile(weekTypeFile)
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal(weeksYaml, wm)
		if err != nil {
			return nil, err
		}
	}

	pm := make(PlayerMap)
	for p, r := range rm {
		var err error
		pm[p], err = NewPlayer(p, r, wm[p])
		if err != nil {
			return nil, err
		}
	}

	return pm, nil
}

// Duplicates returns a list of Players who are duplicates of one another.
func (pm PlayerMap) Duplicates() map[string][]string {

	playerHashes := make(map[uint64][]string)
	for name, player := range pm {
		hash := jody.HashString64("")
		sort.Sort(TeamList(player.remaining))
		for _, team := range player.remaining {
			hash = jody.AddString64(hash, string(team.Name()))
		}
		for _, weektype := range player.weekTypes.sets {
			hash = jody.AddUint64(hash, uint64(weektype))
		}
		if _, ok := playerHashes[hash]; ok {
			playerHashes[hash] = append(playerHashes[hash], name)
		} else {
			playerHashes[hash] = []string{name}
		}
	}

	out := make(map[string][]string)
	for _, duplicates := range playerHashes {
		out[duplicates[0]] = make([]string, 0)
		for _, dup := range duplicates {
			if dup == duplicates[0] {
				continue
			}
			out[duplicates[0]] = append(out[duplicates[0]], dup)
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

func (p Player) String() string {
	return fmt.Sprintf("%s: %v %v\n", p.Name(), p.RemainingTeams(), p.weekTypes.sets)
}
