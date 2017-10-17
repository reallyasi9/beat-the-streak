package main

import (
	"flag"
	"fmt"
	"log"
	"sync"

	"runtime"

	"./bts"
)

var numCPU = runtime.GOMAXPROCS(0)
var b1gTeams = map[string]string{
	"Illinois":       "ILL",
	"Indiana":        "IND",
	"Iowa":           "IOWA",
	"Maryland":       "UMD",
	"Michigan":       "MICH",
	"Michigan State": "MSU",
	"Minnesota":      "MINN",
	"Nebraska":       "NEB",
	"Northwestern":   "NU",
	"Ohio State":     "OSU",
	"Penn State":     "PSU",
	"Purdue":         "PUR",
	"Rutgers":        "RUT",
	"Wisconsin":      "WISC"}
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
var weekNumber = flag.Int("week", -1, "Week `number` [1-13]")
var penalty = flag.Float64("penalty", 1.0, "Penalty `probability` where to begin a linear penalty (to avoid high-probability games in accordance with the tiebreaker rules)")

func main() {
	flag.Parse()

	ratings, err := bts.MakeRatings(*ratingsURL)
	if err != nil {
		panic(err)
	}
	log.Printf("Downloaded ratings %v", ratings)

	bias, stdDev, err := bts.ScrapeParameters(*performanceURL, "Sagarin Points")
	if err != nil {
		panic(err)
	}
	log.Printf("Scraped bias %f, standard dev %f", bias, stdDev)

	schedule, err := bts.MakeSchedule(*scheduleFile)
	if err != nil {
		panic(err)
	}
	log.Printf("Made schedule %v", schedule)

	probs, err := ratings.MakeProbabilities(schedule, bias, stdDev, *penalty)
	if err != nil {
		panic(err)
	}
	log.Printf("Made probabilities %v", probs)

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
	log.Printf("Filtered probabilities: %v", probs)

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
	results := make(map[string]chan bts.Streak)
	var wg sync.WaitGroup
	for user, remainingTeams := range players {
		wg.Add(1)

		teams := make(bts.TeamList, len(remainingTeams))
		for i, t := range remainingTeams {
			teams[i] = bts.Team(t)
		}

		var ddTeam *bts.DoubleDown
		if ddusers[user] {
			ddTeam = bts.BestWeek(teams[0], probs)
			teams = teams[1:]
		}

		streak := bts.Streak{
			Teams:       teams,
			DD:          ddTeam,
			Probability: teams.Probability(probs),
		}

		results[user] = make(chan bts.Streak, 100)
		go func() {
			defer wg.Done()
			streak.Permute(results[user], probs)
		}()
	}

	for user, result := range results {
		wg.Add(1)
		go func(user string, result chan bts.Streak) {
			defer wg.Done()
			best := <-result
			for p := range result {
				if p.Probability > best.Probability {
					best = p
				}
			}

			fmt.Printf("%s", user)
			if _, ok := duplicates[user]; ok {
				fmt.Printf(" (clones %v)", duplicates[user])
			}
			fmt.Println()
			fmt.Println(best.String(probs))
		}(user, result)
	}

	wg.Wait()
}
