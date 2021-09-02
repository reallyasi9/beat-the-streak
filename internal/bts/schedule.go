package bts

import (
	"fmt"
	"io/ioutil"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

// Schedule is a team's schedule for the year.
type Schedule map[Team][]*Game

// MakeSchedule parses a schedule YAML file.
func MakeSchedule(fileName string) (*Schedule, error) {

	schedYaml, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	s := make(map[Team][]string)
	err = yaml.Unmarshal(schedYaml, s)
	if err != nil {
		return nil, err
	}

	// for k, v := range s {
	// 	if len(v) != NGames {
	// 		return nil, fmt.Errorf("schedule for team %s incorrect: expected %d, got %d", k, NGames, len(v))
	// 	}
	// }
	sched := make(Schedule)
	for team, locteams := range s {
		sched[team] = make([]*Game, len(locteams))
		for i, locteam := range locteams {
			loc, team2 := splitLocTeam(locteam)
			sched[team][i] = NewGame(team, team2, loc)
		}
	}

	return &sched, nil
}

// Get a game for a team and week number.
func (s Schedule) Get(t Team, w int) *Game {
	if t == NONE {
		// Picking no team is strange
		return &NULLGAME
	}
	return s[t][w]
}

// NumWeeks returns the number of weeks contained in the schedule.
func (s Schedule) NumWeeks() int {
	for _, v := range s {
		return len(v)
	}
	return 0
}

// FilterWeeks filters the Predictions by removing weeks prior to the given one.
func (s *Schedule) FilterWeeks(w int) {
	if w <= 0 {
		return
	}
	for team := range *s {
		(*s)[team] = (*s)[team][w:]
	}
}

// TeamList generates a list of first-level teams from the schedule.
func (s Schedule) TeamList() TeamList {
	tl := make(TeamList, len(s))
	i := 0
	for t := range s {
		tl[i] = t
		i++
	}
	return tl
}

func splitLocTeam(locTeam string) (RelativeLocation, Team) {
	if locTeam == "BYE" || locTeam == "" {
		return Neutral, BYE
	}
	// Note: this is relative to the schedule team, not the team given here.
	switch locTeam[0] {
	case '@':
		return Away, Team(locTeam[1:])
	case '>':
		return Far, Team(locTeam[1:])
	case '<':
		return Near, Team(locTeam[1:])
	case '!':
		return Neutral, Team(locTeam[1:])
	default:
		return Home, Team(locTeam)
	}
}

func (s Schedule) String() string {
	tl := s.TeamList()
	nW := s.NumWeeks()
	var b strings.Builder

	b.WriteString("      ")
	for week := 0; week < nW; week++ {
		b.WriteString(fmt.Sprintf("%-5d ", week))
	}
	b.WriteString("\n")

	for _, team := range tl {
		b.WriteString(fmt.Sprintf("%4s: ", team))
		for week := 0; week < nW; week++ {
			g := s.Get(team, week)
			extra := ' '
			switch g.LocationRelativeToTeam(0) {
			case Away:
				extra = '@'
			case Far:
				extra = '>'
			case Near:
				extra = '<'
			case Neutral:
				extra = '!'
			}
			if g.Team(1) != BYE {
				b.WriteRune(extra)
			} else {
				b.WriteRune(' ')
			}
			b.WriteString(fmt.Sprintf("%-4s ", g.Team(1)))
		}
		b.WriteString("\n")
	}

	return b.String()
}
