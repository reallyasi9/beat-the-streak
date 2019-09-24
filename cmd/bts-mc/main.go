package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/reallyasi9/beat-the-streak/internal/bts"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
		panic(err)
	}
}

var ratingsURL = flag.String("ratings",
	"http://sagarin.com/sports/cfsend.htm",
	"`URL` of Sagarin ratings for calculating probabilities of win")
var performanceURL = flag.String("performance",
	"http://www.thepredictiontracker.com/ncaaresults.php",
	"`URL` of model performances for calculating probabilities of win")
var scheduleFile = flag.String("schedule",
	"schedule.yaml",
	"YAML `file` containing B1G schedule")
var remainingFile = flag.String("remaining", "remaining.yaml", "YAML `file` containing picks remaining for each contestant")
var weekTypesFile = flag.String("weektypes", "", "YAML `file` containing week types remaining for each contestant")
var weekNumber = flag.Int("week", -1, "Week `number` (starting at 0)")
var nTop = flag.Int("n", 5, "`number` of top probabilities to report for each player to check for better spreads")

var startTime = time.Now()

func main() {
	flag.Parse()

	model, err := bts.MakeGaussianSpreadModel(*ratingsURL, *performanceURL, "Sagarin Points")
	check(err)
	log.Printf("Downloaded model %v", model)

	schedule, err := bts.MakeSchedule(*scheduleFile)
	check(err)
	log.Printf("Made schedule\n%v", schedule)

	predictions := bts.MakePredictions(schedule, *model)
	log.Printf("Made predictions\n%s", predictions)

	// Figure out week types if necessary
	if *weekTypesFile == "" {
		log.Print("No week-type file given, assuming no byes/n-downs")
	}

	players, err := bts.MakePlayers(*remainingFile, *weekTypesFile)
	check(err)
	log.Printf("Made players %v", players)

	log.Printf("The following users have not yet been eliminated: %v", players)

	// Determine week number, if needed
	if *weekNumber < 0 {
		log.Print("Valid week number not given: attempting to determine week number from input data")
		*weekNumber = determineWeekNumber(players, schedule)
	}
	// err = validateWeekNumber(*weekNumber, players)
	// check(err)
	log.Printf("Week number %d", *weekNumber)

	schedule.FilterWeeks(*weekNumber)
	log.Printf("Filtered schedule:\n%s", schedule)

	predictions.FilterWeeks(*weekNumber)
	log.Printf("Filtered predictions:\n%s", predictions)

	// Here we go.
	// Find the unique users.
	duplicates := players.Duplicates()
	log.Println("The following users are clones of one another:")
	for user, clones := range duplicates {
		log.Printf("%s clones %v", user, clones)
		for _, clone := range clones {
			delete(players, clone)
		}
	}

	// Loop through the unique users
	playerItr := playerIterator(players)

	// Loop through streaks
	ppts := perPlayerTeamStreaks(playerItr, predictions)

	// Update best
	bestStreaks := calculateBestStreaks(ppts)

	// Collect by player
	streakOptions := collectByPlayer(bestStreaks, players, predictions, schedule)

	// Print results
	for _, streak := range streakOptions {
		j, _ := json.Marshal(streak)
		fmt.Println(string(j))
	}

}

// StreakMap is a simple map of player names to streaks
type streakMap map[playerTeam]streakProb

type streakProb struct {
	streak *bts.Streak
	prob   float64
	spread float64
}

type playerTeam struct {
	player string
	team   bts.Team
}

func (sm *streakMap) update(player string, team bts.Team, spin streakProb) {
	pt := playerTeam{player: player, team: team}
	bestp := math.Inf(-1)
	bests := math.Inf(-1)
	if sp, ok := (*sm)[pt]; ok {
		bestp = sp.prob
		bests = sp.spread
	}
	if spin.prob > bestp || (spin.prob == bestp && spin.spread > bests) {
		(*sm)[pt] = streakProb{streak: spin.streak, prob: spin.prob, spread: spin.spread}
	}
}

func (sm streakMap) getBest(player string) streakProb {
	bestp := math.Inf(-1)
	bests := math.Inf(-1)
	bestt := bts.BYE
	for pt, sp := range sm {
		if pt.player != player {
			continue
		}
		if sp.prob > bestp || (sp.prob == bestp && sp.spread > bests) {
			bestt = pt.team
		}
	}
	return sm[playerTeam{player: player, team: bestt}]
}

type playerTeamStreakProb struct {
	player     *bts.Player
	team       bts.Team
	streakProb streakProb
}

func playerIterator(pm bts.PlayerMap) <-chan *bts.Player {
	out := make(chan *bts.Player)

	go func() {
		defer close(out)

		for _, player := range pm {
			out <- player
		}
	}()

	return out
}

func perPlayerTeamStreaks(ps <-chan *bts.Player, predictions *bts.Predictions) <-chan playerTeamStreakProb {

	out := make(chan playerTeamStreakProb)

	go func() {
		var wg sync.WaitGroup

		for p := range ps {
			wg.Add(1)

			go func(p *bts.Player) {
				for weekOrder := range p.WeekTypeIterator() {
					for teamOrder := range p.RemainingIterator() {
						streak := bts.NewStreak(p.RemainingTeams(), weekOrder, teamOrder)
						prob, spread := bts.SummarizeStreak(predictions, streak)
						if prob <= 0 {
							// Ignore streaks that guarantee a loss.
							continue
						}
						for _, team := range streak.GetWeek(0) {
							sp := streakProb{streak: streak, prob: prob, spread: spread}
							out <- playerTeamStreakProb{player: p, team: team, streakProb: sp}
						}
					}
				}
				wg.Done()
			}(p)

		}
		wg.Wait()
		close(out)
	}()

	return out
}

func calculateBestStreaks(ppts <-chan playerTeamStreakProb) <-chan streakMap {
	out := make(chan streakMap)

	sm := make(streakMap)
	go func() {
		defer close(out)

		for ptsp := range ppts {
			sm.update(ptsp.player.Name(), ptsp.team, ptsp.streakProb)
		}

		out <- sm
	}()

	return out
}

func collectByPlayer(sms <-chan streakMap, players bts.PlayerMap, predictions *bts.Predictions, schedule *bts.Schedule) []PlayerResults {

	// Collect streak options by player
	soByPlayer := make(map[string][]StreakOption)
	for sm := range sms {

		for pt, sp := range sm {

			firstTeam := pt.team
			cProb, cSpread := bts.AccumulateStreak(predictions, sp.streak)
			prob := sp.prob
			spread := sp.spread

			weeks := make([]Week, sp.streak.NumWeeks())
			for iweek := 0; iweek < sp.streak.NumWeeks(); iweek++ {

				seasonWeek := iweek + *weekNumber
				weekProbability := 1.
				weekSpread := 0.

				picks := make([]Pick, 0)
				for _, team := range sp.streak.GetWeek(iweek) {
					probability := predictions.GetProbability(team, iweek)
					spread := predictions.GetSpread(team, iweek)
					opponent := schedule.Get(team, iweek).Team(1)

					picks = append(picks, Pick{Selected: team, Opponent: opponent, Probability: probability, Spread: spread})

					weekProbability *= probability
					weekSpread += spread
				}

				weeks[iweek] = Week{Picks: picks, SeasonWeek: seasonWeek, Probability: weekProbability, Spread: weekSpread}

			}

			so := StreakOption{FirstSelected: firstTeam, Weeks: weeks, CumulativeProbability: cProb, CumulativeSpread: cSpread, Probability: prob, Spread: spread}
			if sos, ok := soByPlayer[pt.player]; ok {
				soByPlayer[pt.player] = append(sos, so)
			} else {
				soByPlayer[pt.player] = []StreakOption{so}
			}

		}

	}

	// Run through players and calculate best option
	prs := make([]PlayerResults, 0)
	for player, streakOptions := range soByPlayer {

		if len(streakOptions) == 0 {
			continue
		}

		sort.Sort(ByProbDesc(streakOptions))

		bestSelection := make([]bts.Team, 0)
		for _, pick := range streakOptions[0].Weeks[0].Picks {
			bestSelection = append(bestSelection, pick.Selected)
		}
		bestProb := streakOptions[0].Probability
		bestSpread := streakOptions[0].Spread

		prs = append(prs, PlayerResults{
			Player:               player,
			StartingWeek:         *weekNumber,
			CalculationStartTime: startTime,
			CalculationEndTime:   time.Now(),
			RemainingTeams:       players[player].RemainingTeams(),
			RemainingWeekTypes:   players[player].RemainingWeekTypes(),
			BestSelection:        bestSelection,
			BestProbability:      bestProb,
			BestSpread:           bestSpread,
			StreakOptions:        streakOptions,
		})
	}

	return prs
}

func determineWeekNumber(players bts.PlayerMap, schedule *bts.Schedule) int {
	guess := -1
	for name, player := range players {
		thisGuess := player.RemainingWeeks()
		if guess >= 0 && thisGuess != guess {
			panic(fmt.Errorf("player %s has an invalid number of weeks remaining: expected %d, found %d", name, thisGuess, guess))
		}
		guess = thisGuess
	}
	week := schedule.NumWeeks() - guess
	return week
}
