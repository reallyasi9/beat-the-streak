package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	//"github.com/sethgrid/multibar"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	//"sync"
	"math"
)

var numCPU = runtime.GOMAXPROCS(0)
var dataFile = flag.String("probs", "", "CSV `file` containing probabilities of win")
var remFile = flag.String("remaining", "", "CSV `file` containing picks remaining for each contestant")
var weekNumber = flag.Int("week", -1, "Week `number` [1-13]")

func main() {

	pReader, rReader, err := parseFlags()

	if err != nil {
		flag.PrintDefaults()
		fmt.Printf("error parsing flags: %s\n", err)
		os.Exit(-1)
	}

	// Parse out the probabilities
	probs, err := parseProbs(pReader)
	if err != nil {
		fmt.Printf("error parsing probability file: %s\n", err)
		os.Exit(-1)
	}

	// Parse out the remaining teams
	remaining, err := parseRemaining(rReader)
	if err != nil {
		fmt.Printf("error parsing remaining file: %s\n", err)
		os.Exit(-1)
	}

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
	uniqueUsers := make(map[string]selection)
	for u, r := range remaining {
		if len(uniqueUsers) == 0 {
			uniqueUsers[u] = r
			continue
		}

		found := false
		for uu, ur := range uniqueUsers {
			if r.equals(ur) {
				fmt.Printf("%s <- %s are the same\n", uu, u)
				found = true
				break
			}
		}

		if !found {
			uniqueUsers[u] = r
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

			//wg := &sync.WaitGroup{}
			//wg.Add(numCPU)

			//bc, _ := multibar.New()
			//go bc.Listen()

			for i := 0; i < nGoes; i++ {
				//bc.MakeBar(pPerThread, fmt.Sprintf("permutations %d/%d", i+1, numCPU))
				go permute(i, pPerThread, remainingTeams, probs, "", -1, results)
			}
		}

		for i := 0; i < nGoes; i++ {
			bestPerm.UpdateGT(<-results)
		}

		fmt.Printf("--%s--\n", user)
		pb.PrintProbs(bestPerm)
	}
}

func parseFlags() (*csv.Reader, *csv.Reader, error) {
	flag.Parse()
	if *dataFile == "" {
		return nil, nil, fmt.Errorf("probs flag required")
	}

	if *remFile == "" {
		return nil, nil, fmt.Errorf("remaining flag required")
	}

	if *weekNumber < 1 || *weekNumber > 13 {
		return nil, nil, fmt.Errorf("week number must be specified and must be in the range [1,13]")
	}

	csvFile, err := os.Open(*dataFile)
	if err != nil {
		return nil, nil, err
	}
	pReader := csv.NewReader(csvFile)

	csvFile2, err := os.Open(*remFile)
	if err != nil {
		return pReader, nil, err
	}
	rReader := csv.NewReader(csvFile2)

	return pReader, rReader, nil
}

// Slices are passed by reference
func parseProbRow(row []string) (string, []float64, error) {
	var err error
	team := row[0]
	probs := make([]float64, len(row)-1)
	for i, rec := range row[1:] {
		if rec == "#N/A" {
			continue // defaults to zero
		}
		probs[i], err = strconv.ParseFloat(rec, 64)
		if err != nil {
			return team, probs, err
		}
	}
	return team, probs, nil
}

func parseRemRow(row []string) (string, []bool, error) {
	//var err error
	team := row[0]
	rem := make([]bool, len(row)-2)
	for i, val := range row[1 : len(row)-1] {
		if val == team {
			rem[i] = true
		} else if val != "" {
			// Done now
			//err = fmt.Errorf("unrecognized team remaining in row %v: %s", i, val)
			return team, rem, nil
		}
		// defaults to false
	}
	return team, rem, nil
}

func parseProbs(r *csv.Reader) (probabilityMap, error) {

	// Throw away the first row (week numbers)
	_, err := r.Read()
	if err != nil {
		fmt.Printf("error reading week numbers: %s\n", err)
		os.Exit(-1)
	}

	// Parse remaining data and store it
	var teams []string
	p := make(probabilityMap)
	row, err := r.Read()
	for ; err != io.EOF; row, err = r.Read() {
		if err != nil {
			return nil, err
		}

		if len(row) == 0 {
			break
		}

		team, prob, e := parseProbRow(row)
		if e != nil {
			return nil, e
		}

		teams = append(teams, team)
		p[team] = prob
	}

	if len(teams) != len(p) {
		err = fmt.Errorf("error parsing data : %d teams != %d rows, meaning a team was repeated in the probability file", len(teams), len(p))
		return nil, err
	}

	// Make sure the number of weeks is consistent across teams
	nWeeks := 0
	for k, v := range p {
		if nWeeks == 0 {
			nWeeks = len(v)
			continue
		}
		if len(v) != nWeeks {
			err = fmt.Errorf("error parsing data : weeks for team %s (%d) does not match other teams (%d)\n", k, len(v), nWeeks)
			return nil, err
		}
	}

	return p, nil
}

func parseRemaining(r *csv.Reader) (remainingMap, error) {

	// The first row are the contestents
	row, err := r.Read()
	if err != nil {
		e := fmt.Errorf("error reading users: %s\n", err)
		return nil, e
	}

	users := row[1 : len(row)-1]

	// Eject the next row as it is a relic of times past
	r.Read()

	// Parse remaining data and store it
	rem := make(remainingMap)
	row, err = r.Read()
	for ; err != io.EOF; row, err = r.Read() {
		if err != nil {
			return nil, err
		}

		if len(row) < len(users)+2 {
			// There's a row at the end that is not useful
			break
		}

		team, remaining, e := parseRemRow(row)
		if e != nil {
			return nil, e
		}

		for i, userRem := range remaining {
			if userRem {
				rem[users[i]] = append(rem[users[i]], team)
			}
		}

	}

	if len(users) != len(rem) {
		err = fmt.Errorf("error parsing data : %d users != %d rows, meaning a user was repeated in the remaining file", len(users), len(rem))
		return nil, err
	}

	return rem, nil
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
