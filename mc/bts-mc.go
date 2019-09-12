package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"sync"

	"runtime"

	"../bts"
)

var numCPU = runtime.GOMAXPROCS(0)
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
var weekTypesFile = flag.String("weektypes", "weektypes_remaining.yaml", "YAML `file` containing week types remaining for each contestant")
var weekNumber = flag.Int("week", 0, "Week `number` (starting at 0)")
var nTop = flag.Int("n", 5, "`number` of top probabilities to report for each player to check for better spreads")

func main() {
	flag.Parse()

	model, err := bts.MakeGaussianSpreadModel(*ratingsURL, *performanceURL, "Sagarin Points")
	if err != nil {
		panic(err)
	}
	log.Printf("Downloaded model %v", model)

	schedule, err := bts.MakeSchedule(*scheduleFile)
	if err != nil {
		panic(err)
	}
	log.Printf("Made schedule\n%v", schedule)

	predictions := bts.MakePredictions(schedule, *model)
	log.Printf("Made predictions\n%s", predictions)

	players, err := bts.MakePlayers(*remainingFile, *weekTypesFile)
	if err != nil {
		panic(err)
	}
	log.Printf("Made players %v", players)

	// Determine week number, if needed
	if *weekNumber < 0 {
		panic(fmt.Errorf("week number must be greater than or equal to zero, got %d", *weekNumber))
	}
	log.Printf("Week number %d", *weekNumber)

	// Determine double-down users

	log.Printf("The following users have not yet been eliminated: %v", players)

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
	for player := range playerItr {
		fmt.Println(player)
		streaks := perPlayerTeamStreaks(player, predictions)
		streakMaps := calculateBestStreaks(streaks)

		for streak := range streakMaps {
			for pt, sp := range streak {
				fmt.Printf("%v: %f(%f)\n%v\n", pt.player, sp.prob, sp.spread, sp.streak)
				// fmt.Println(sp)
			}
		}
	}
	// results := make(chan playerResult, len(players))
	// jobs := make(chan namedPlayer, len(players))

	// for i := 0; i < runtime.NumCPU(); i++ {
	// 	go worker(i, jobs, results, probs, *nTop)
	// }

	// for user, remainingTeams := range players {
	// 	jobs <- namedPlayer{Player: user, DD: ddusers[user], Teams: remainingTeams}
	// }
	// close(jobs)

	// // Drain the results now
	// for range players {
	// 	result := <-results
	// 	fmt.Printf("%s", result.Player)
	// 	if _, ok := duplicates[result.Player]; ok {
	// 		fmt.Printf(" (clones %v)", duplicates[result.Player])
	// 	}
	// 	fmt.Println()
	// 	for _, res := range result.Result {
	// 		if res.Streak == nil {
	// 			continue
	// 		}
	// 		fmt.Println(res.Streak.String(probs, spreads, *weekNumber))
	// 	}
	// }

	// close(results)

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

func mergeWait(cs ...<-chan int) <-chan int {
	out := make(chan int)
	var wg sync.WaitGroup
	wg.Add(len(cs))
	for _, c := range cs {
		go func(c <-chan int) {
			for v := range c {
				out <- v
			}
			wg.Done()
		}(c)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

type playerTeamStreakProb struct {
	player     string
	team       bts.Team
	streakProb streakProb
}

func playerIterator(pm bts.PlayerMap) <-chan bts.Player {
	out := make(chan bts.Player)

	go func() {
		defer close(out)

		for _, player := range pm {
			out <- player
		}
	}()

	return out
}

func perPlayerTeamStreaks(p bts.Player, predictions *bts.Predictions) <-chan playerTeamStreakProb {
	out := make(chan playerTeamStreakProb)

	go func() {
		defer close(out)

		for weekOrder := range p.WeekTypeIterator() {
			for teamOrder := range p.RemainingIterator() {
				streak := bts.NewStreak(p.RemainingTeams(), weekOrder, teamOrder)
				prob, spread := bts.SummarizeStreak(predictions, streak)
				for _, team := range streak.GetWeek(0) {
					sp := streakProb{streak: streak, prob: prob, spread: spread}
					out <- playerTeamStreakProb{player: p.Name(), team: team, streakProb: sp}
				}
			}
		}
	}()

	return out
}

func calculateBestStreaks(ppts <-chan playerTeamStreakProb) <-chan streakMap {
	out := make(chan streakMap)

	sm := make(streakMap)
	go func() {
		defer close(out)

		for ptsp := range ppts {
			sm.update(ptsp.player, ptsp.team, ptsp.streakProb)
		}

		out <- sm
	}()

	return out
}
