package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"

	"runtime"

	"../bts"
	"github.com/atgjack/prob"
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
	"YAML `file` containing schedule of all pick-your-pony contenders")
var nMC = flag.Int("n", 1000000, "`number` of Monte Carlo simulations to run for each team")
var hyperVariance = flag.Float64("var",
	4.723,
	"Assumed prior `variance` of Sagarin ratings")

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

	// I don't need all the ratings, only those that appear in the schedule.
	// Remove the teams that don't matter
	markedTeams := make(map[bts.Team]bool)
	for t1, sched := range schedule {
		markedTeams[t1] = true
		for _, t2 := range sched {
			if t2 == "" {
				continue
			}
			if t2[0] == '!' || t2[0] == '@' || t2[0] == '<' || t2[0] == '>' {
				t2 = t2[1:]
			}
			markedTeams[t2] = true
		}
	}
	for t := range ratings {
		if _, ok := markedTeams[t]; !ok {
			delete(ratings, t)
		}
	}

	// Loop through the teams
	results := make(chan teamResults, len(schedule))
	jobs := make(chan bts.Team, len(schedule))

	for i := 0; i < runtime.NumCPU()+1; i++ {
		go worker(i, jobs, results, schedule, ratings, bias, stdDev, *hyperVariance, *nMC)
	}

	for team := range schedule {
		jobs <- team
	}
	close(jobs)

	// Print the table header
	fmt.Print(" Team      Wins: ")
	for i := 0; i < 13; i++ {
		fmt.Printf(" %5d ", i)
	}
	fmt.Println()

	// Drain the results now
	for range schedule {
		result := <-results
		t := result.Team
		if len(t) > 15 {
			t = t[:15]
		}
		fmt.Printf(" %15s ", t)
		for _, p := range result.WinProbabilities {
			fmt.Printf(" %5.3f ", p)
		}
		fmt.Println()
	}

	close(results)

}

type teamResults struct {
	Team             bts.Team
	WinProbabilities []float64
}

func worker(i int, jobs <-chan bts.Team, results chan<- teamResults, s bts.Schedule, r bts.Ratings, bias float64, std float64, hypervariance float64, nmc int) {
	for t := range jobs {
		results <- teamResults{Team: t, WinProbabilities: simulateWins(t, s, r, bias, std, hypervariance, nmc)}
	}
}

func simulateWins(team bts.Team, s bts.Schedule, r bts.Ratings, bias, std, hypervariance float64, nmc int) []float64 {
	winHist := make([]int, len(s[team]))

	ratingNormal, err := prob.NewNormal(0, hypervariance)
	if err != nil {
		panic(err)
	}

	for i := 0; i < nmc; i++ {
		// nudge ratings by a random amount
		myRatings := make(bts.Ratings)
		for t, rating := range r {
			myRatings[t] = rating + ratingNormal.Random()
		}

		// calculate probabilities from nudged ratings
		probs, _, err := myRatings.MakeProbabilities(s, bias, std)
		if err != nil {
			panic(err)
		}

		// Simulate wins from probabilities
		wins := 0
		for _, prob := range probs[team] {
			if rand.Float64() < prob {
				wins++
			}
		}

		// Count it
		winHist[wins]++
	}

	// Normalize win counts
	out := make([]float64, len(winHist))
	for i, win := range winHist {
		out[i] = float64(win) / float64(nmc)
	}

	return out
}
