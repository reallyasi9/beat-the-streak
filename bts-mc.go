package main

import (
	"flag"
	"fmt"

	"math"
	"os"
	"runtime"
	"sort"

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

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	flag.Parse()

	ratings, err := bts.MakeRatings(*ratingsURL)
	checkErr(err)
	bias, stdDev, err := bts.ScrapeParameters(*performanceURL, "Sagarin Points")
	checkErr(err)
	schedule, err := bts.MakeSchedule(*scheduleFile)
	checkErr(err)
	probs, err := ratings.MakeProbabilities(schedule, bias, stdDev, *penalty)
	checkErr(err)
	remaining, err := makePlayers(*remainingFile)
	checkErr(err)

	// You can have at most 1 more team remaining than weeks remaining, but can
	// never have fewer than that.
	ddusers, err := remaining.TrimUsers(*weekNumber)
	if err != nil {
		fmt.Printf("error trimming users: %s\n", err)
		os.Exit(-1)
	}

	users := remaining.Users()
	teams := probs.Teams()

	fmt.Printf("The following users have not yet been eliminated:\n%v\n", users)

	var dduserSlice []string
	for user, tf := range ddusers {
		if tf {
			dduserSlice = append(dduserSlice, user)
		}
	}
	fmt.Printf("The following users still have their double-down remaining:\n%v\n", dduserSlice)

	fmt.Printf("Teams: %v\n", teams)
	fmt.Printf("Probabilities:\n%v\n", probs)

	err = probs.FilterWeeks(*weekNumber)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	fmt.Printf("Filtered Probabilities:\n%v\n", probs)

	// Here we go.
	// Find the unique remaining teams.
	uniqueUsers, uniques := remaining.UniqueUsers()
	fmt.Println("The following users are clones of one another:")
	for uu, ou := range uniques {
		if len(ou) == 0 {
			fmt.Printf("%s (unique)\n%v\n", uu, uniqueUsers[uu])
		} else {
			fmt.Printf("%s cloned by %s\n%v\n", uu, ou, uniqueUsers[uu])
		}
	}

	// Loop through the unique users
	for user, remainingTeams := range uniqueUsers {

		pb, err := probs.CopyWithTeams(remainingTeams)
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}

		// For permutations to work properly, these should be sorted
		sort.Strings(remainingTeams)

		// This is the best I can do
		var bestPerm orderPerm

		// These are the results from my goroutines
		results := make(chan orderPerm)

		// I could make this more complicated by closing the channel, but I will instead just count the goroutines
		var nGoes int

		if ddusers[user] {
			nGoes = len(remainingTeams)

			// Each dd team gets its own goroutine
			for _, ddt := range remainingTeams {
				_, ddw := maxFloat64(pb[ddt])

				teamsAfterDD, _ := remainingTeams.CopyWithoutTeam(ddt)

				pPerThread := math.MaxInt32
				go permute(0, pPerThread, teamsAfterDD, pb, ddt, ddw, results)

			}

		} else {

			// If there are more teams remaining than cores, then the number of permutations
			// will always be divisible evenly by the remaining cores.  If not, then don't
			// bother too much to try to fill all cores with goroutines, because your overhead
			// is going to kill you anyway.
			if len(remainingTeams) > numCPU {
				nGoes = numCPU
			} else {
				nGoes = len(remainingTeams)
			}

			// Divy up the permutations
			nPermutations := intFactorial(len(remainingTeams))
			pPerThread := nPermutations / nGoes

			for i := 0; i < nGoes; i++ {
				go permute(i, pPerThread, remainingTeams, probs, "", -1, results)
			}
		}

		for i := 0; i < nGoes; i++ {
			bestPerm.UpdateGT(<-results)
		}

		fmt.Printf("-- %s ", user)
		if len(uniques[user]) == 0 {
			fmt.Print("(unique)\n")
		} else {
			fmt.Printf("cloned by %s\n", uniques[user])
		}
		pb.PrintProbs(bestPerm)
	}
}
