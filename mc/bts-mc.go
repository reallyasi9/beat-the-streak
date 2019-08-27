package main

import (
	"flag"
	"fmt"
	"log"

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
var weekNumber = flag.Int("week", -1, "Week `number` (starting at 1)")
var nTop = flag.Int("n", 5, "`number` of top probabilities to report for each player to check for better spreads")

func main() {
	flag.Parse()

	ratings, edge, err := bts.MakeRatings(*ratingsURL)
	if err != nil {
		panic(err)
	}
	log.Printf("Downloaded ratings %v", ratings)
	log.Printf("Parsed home edge %f", edge)

	bias, stdDev, err := bts.ScrapeParameters(*performanceURL, "Sagarin Points")
	if err != nil {
		panic(err)
	}
	log.Printf("Scraped bias %f, standard dev %f", bias, stdDev)
	log.Printf("Combined bias %f", bias+edge)

	schedule, err := bts.MakeSchedule(*scheduleFile)
	if err != nil {
		panic(err)
	}
	log.Printf("Made schedule %v", schedule)

	probs, spreads, err := ratings.MakeProbabilities(schedule, bias+edge, stdDev)
	if err != nil {
		panic(err)
	}
	log.Printf("Made probabilities\n%s", probs)
	log.Printf("Made spreads\n%s", spreads)

	players, err := bts.MakePlayers(*remainingFile)
	if err != nil {
		panic(err)
	}
	log.Printf("Made players %v", players)

	// Determine week number, if needed
	if *weekNumber == -1 {
		week, err2 := players.InferWeek()
		if err2 != nil {
			panic(err2)
		}
		*weekNumber = week
	}
	log.Printf("Week number %d", *weekNumber)

	// Determine double-down users
	ddusers, err := players.DoubleDownRemaining(*weekNumber)
	if err != nil {
		panic(err)
	}

	log.Printf("The following users have not yet been eliminated: %v", players)
	log.Printf("The following users still have their double-down remaining: %v", bts.SliceMap(ddusers))

	probs.FilterWeeks(*weekNumber)
	spreads.FilterWeeks(*weekNumber)
	log.Printf("Filtered probabilities:\n%s", probs)
	log.Printf("Filtered spreads:\n%s", spreads)

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
	results := make(chan playerResult, len(players))
	jobs := make(chan namedPlayer, len(players))

	for i := 0; i < runtime.NumCPU(); i++ {
		go worker(i, jobs, results, probs, *nTop)
	}

	for user, remainingTeams := range players {
		jobs <- namedPlayer{Player: user, DD: ddusers[user], Teams: remainingTeams}
	}
	close(jobs)

	// Drain the results now
	for range players {
		result := <-results
		fmt.Printf("%s", result.Player)
		if _, ok := duplicates[result.Player]; ok {
			fmt.Printf(" (clones %v)", duplicates[result.Player])
		}
		fmt.Println()
		for _, res := range result.Result {
			if res.Streak == nil {
				continue
			}
			fmt.Println(res.Streak.String(probs, spreads, *weekNumber))
		}
	}

	close(results)

}

type playerResult struct {
	Player string
	Result bts.StreaksByProb
}

type namedPlayer struct {
	Player string
	DD     bool
	Teams  bts.Player
}

func worker(i int, jobs <-chan namedPlayer, results chan<- playerResult, probs bts.Probabilities, nTop int) {
	for p := range jobs {
		results <- playerResult{Player: p.Player, Result: p.Teams.BestStreaks(probs, p.DD, nTop)}
	}
}
