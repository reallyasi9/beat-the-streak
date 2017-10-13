package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"

	"github.com/atgjack/prob"

	yaml "gopkg.in/yaml.v2"

	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
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

	ratings, err := makeRatings(*ratingsURL)
	checkErr(err)
	bias, stdDev, err := scrapeParameters(*performanceURL, "Sagarin Points")
	checkErr(err)
	schedule, err := makeSchedule(*scheduleFile)
	checkErr(err)
	probs, err := ratings.makeProbabilities(schedule, bias, stdDev, *penalty)
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

func getURLBody(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

type ratings map[string]float64

func (r ratings) makeProbabilities(s schedule, bias, stdDev, penalty float64) (probabilityMap, error) {
	normal := prob.Normal{Mu: 0, Sigma: stdDev}

	p := make(probabilityMap)

	for team1, sched := range s {
		p[team1] = make([]float64, 13)
		rating1, ok := r[team1]
		if !ok {
			return nil, fmt.Errorf("team %s not in ratings", team1)
		}
		for i, team2 := range sched {
			if team2 == "" {
				p[team1][i] = 0.
				continue
			}
			t2home := team2[0] == '@'
			t2close := team2[0] == '>'
			t1close := team2[0] == '<'
			neutral := team2[0] == '!'
			if t2home || t2close || t1close || neutral {
				team2 = string(team2[1:])
			}
			rating2, ok := r[team2]
			if !ok {
				return nil, fmt.Errorf("team %s (opponent of %s in week %d) not in ratings", team2, team1, i+1)
			}
			spread := rating1 - rating2
			if t2home {
				spread -= bias
			} else if t2close {
				spread -= bias / 2
			} else if t1close {
				spread += bias / 2
			} else if !neutral {
				spread += bias
			}
			rawp := normal.Cdf(spread)
			capp := rawp
			if rawp > penalty && penalty != 1. {
				// Above penalty, the distribution is linear, matching value to the y=x diagonal.
				denom := penalty - 1
				b := penalty / denom
				capp = b*rawp - b
			}
			p[team1][i] = capp
		}
	}

	return p, nil
}

func makeRatings(url string) (ratings, error) {
	body, err := getURLBody(url)
	if err != nil {
		return nil, err
	}

	ratingsRegex := regexp.MustCompile("<font color=\"#000000\">\\s+\\d+\\s+(.*?)\\s+[A]+\\s*=<.*?<font color=\"#0000ff\">\\s*([\\-0-9.]+)")
	ratingsStr := ratingsRegex.FindAllStringSubmatch(string(body), -1)
	if ratingsStr == nil {
		return nil, fmt.Errorf("unable to parse any ratings from %s", url)
	}

	r := make(ratings)
	for _, matches := range ratingsStr {
		rval, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			return nil, err
		}
		r[matches[1]] = rval
	}

	return r, nil
}

func scrapeParameters(url string, modelName string) (float64, float64, error) {
	body, err := getURLBody(url)
	if err != nil {
		return 0., 0., err
	}

	perfRegex := regexp.MustCompile(fmt.Sprintf("%s</font>.*?>[\\-0-9.]+<.*?>[\\-0-9.]+<.*?>[\\-0-9.]+<.*?>([\\-0-9.]+)<.*?>([\\-0-9.]+)<", modelName))
	perfStr := perfRegex.FindSubmatch(body)
	if perfStr == nil {
		return 0., 0., fmt.Errorf("unable to parse bais and mean squared error for model %s from %s", modelName, url)
	}
	bias, err := strconv.ParseFloat(string(perfStr[1]), 64)
	if err != nil {
		return 0., 0., err
	}
	mse, err := strconv.ParseFloat(string(perfStr[2]), 64)
	if err != nil {
		return 0., 0., err
	}
	std := math.Sqrt(mse - bias*bias)
	return bias, std, nil
}

type schedule map[string][]string

func makeSchedule(fileName string) (schedule, error) {

	schedYaml, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	s := make(schedule)
	err = yaml.Unmarshal(schedYaml, s)
	if err != nil {
		return nil, err
	}

	for k, v := range s {
		if len(v) != 13 {
			return nil, fmt.Errorf("schedule for team %s incorrect: expected %d, got %d", k, 13, len(v))
		}
	}

	return s, nil
}

func makePlayers(playerFile string) (remainingMap, error) {
	playerYaml, err := ioutil.ReadFile(playerFile)
	if err != nil {
		return nil, err
	}

	rm := make(remainingMap)
	err = yaml.Unmarshal(playerYaml, rm)
	if err != nil {
		return nil, err
	}

	return rm, nil
}

func maxFloat64(s []float64) (m float64, i int) {
	i = -1
	m = -math.MaxFloat64
	for j, v := range s {
		if v > m {
			i = j
			m = v
		}
	}
	return m, i
}

// https://github.com/cznic/mathutil/blob/master/permute.go
// Generate the next permutation of data if possible and return true.
// Return false if there is no more permutation left.
// Based on the algorithm described here:
// http://en.wikipedia.org/wiki/Permutation#Generation_in_lexicographic_order
func permutationNext(data sort.Interface) bool {
	var k, l int
	for k = data.Len() - 2; ; k-- { // 1.
		if k < 0 {
			return false
		}

		if data.Less(k, k+1) {
			break
		}
	}
	for l = data.Len() - 1; !data.Less(k, l); l-- { // 2.
	}
	data.Swap(k, l)                             // 3.
	for i, j := k+1, data.Len()-1; i < j; i++ { // 4.
		data.Swap(i, j)
		j--
	}
	return true
}

func permute(i int, pPerThread int, remainingTeams selection, probs probabilityMap, ddteam string, ddweek int, results chan orderPerm) {
	// startTime := float64(time.Now().UnixNano()) / 1000000000.

	thisSel := make(selection, len(remainingTeams))
	copy(thisSel, remainingTeams)

	ddProb := 1.
	if ddweek >= 0 {
		ddProb = probs[ddteam][ddweek]
	}

	// skip!
	for nSkip := 0; nSkip < pPerThread*i; nSkip++ {
		permutationNext(thisSel)
	}

	bestProb, _ := probs.TotalProb(thisSel)
	bestPerm := orderPerm{bestProb, thisSel, ddteam, ddweek}
	//fmt.Printf("%d Selection %v Prob (%f)\n", i, bestSel, bestProb)

	for j := 0; j < pPerThread && permutationNext(thisSel); j++ {
		// thisTime := float64(time.Now().UnixNano()) / 1000000000.
		//bc.Bars[i].Update(j)
		totalProb, _ := probs.TotalProb(thisSel)
		totalProb *= ddProb
		if totalProb > bestPerm.prob {
			bestPerm.prob = totalProb
			bestPerm.perm = thisSel.clone()
			bestPerm.ddteam = ddteam
			bestPerm.ddweek = ddweek
			// fmt.Printf("%d,%d,%f,%f,%s\n", i, j, thisTime-startTime, totalProb, bestPerm)
		}
	}

	results <- bestPerm
	//wg.Done()
}

// Overflows when n > 15, so let's hope the B1G doesn't expand...
func intFactorial(n int) int {
	if n <= 1 {
		return 1
	}
	return n * intFactorial(n-1)
}
