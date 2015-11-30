package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"github.com/fighterlyt/permutation"
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
var dataFile = flag.String("data", "", "CSV file containing probabilities of win")
var weekNumber = flag.Int("week", -1, "Week number (defaults to inferring from remaining teams)")
var remainingTeams selection

func init() {
	flag.Var(&remainingTeams, "remaining", "comma-separated list of remaining teams")
}

func main() {

	reader, err := parseFlags()

	if err != nil {
		fmt.Printf("error parsing flags : %s\n", err)
		os.Exit(-1)
	}

	// Throw away the first row (headers)
	_, err = reader.Read()
	if err != nil {
		fmt.Printf("error reading first row : %s\n", err)
		os.Exit(-1)
	}

	// Parse remaining data and store it
	var teams []string
	probs := make(probabilityMap)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("error reading data : %s\n", err)
			os.Exit(-1)
		}

		if len(row) == 0 {
			break
		}

		team, prob, err := parseRow(row)
		if err != nil {
			fmt.Printf("error parsing row : %s\n", err)
			os.Exit(-1)
		}

		teams = append(teams, team)
		probs[team] = prob
	}

	if len(teams) != len(probs) {
		fmt.Printf("error parsing data : %d teams != %d rows\n", len(teams), len(probs))
		os.Exit(-1)
	}

	// Make sure the number of weeks is consistent across teams
	nWeeks := int(0)
	for k, v := range probs {
		if nWeeks == 0 {
			nWeeks = len(v)
			continue
		}
		if len(v) != nWeeks {
			fmt.Printf("error parsing data : weeks for team %s (%d) does not match other teams (%d)\n", k, len(v), nWeeks)
			os.Exit(-1)
		}
	}

	// Can't be predicting past the end of the season
	if *weekNumber > nWeeks {
		fmt.Printf("error parsing week : only %d weeks in data, week %d requested\n", nWeeks, *weekNumber)
		os.Exit(-1)
	}

	// Default remaining teams to all teams
	if len(remainingTeams) == 0 {
		remainingTeams = teams
	}

	// Infer week number
	doubleDown := false
	if *weekNumber <= 0 {
		if len(remainingTeams) <= len(teams)-2 {
			fmt.Println("warning : inferring week number may miss double-down selection, specify week flag to fix")
		}
		if len(remainingTeams) == 1 {
			doubleDown = false
			*weekNumber = nWeeks
		} else {
			*weekNumber = len(teams) - len(remainingTeams) + 1
			doubleDown = true
		}
	} else {
		if len(remainingTeams) > nWeeks-*weekNumber+2 {
			fmt.Printf("error parsing remaining : not enough weeks remaining (%d) to use remaining teams (%d)\n", nWeeks-*weekNumber+1, len(remainingTeams))
			os.Exit(-1)
		}
		if len(remainingTeams) < nWeeks-*weekNumber+1 {
			fmt.Printf("error parsing remaining : not enough teams remaining (%d) to fill remaining weeks (%d)\n", len(remainingTeams), nWeeks-*weekNumber+1)
			os.Exit(-1)
		}
		if len(remainingTeams) == nWeeks-*weekNumber+2 {
			doubleDown = true
		}
	}

	fmt.Printf("CPUs: %d\ndataFile: %s\nremainingTeams: %s\n",
		numCPU, *dataFile, remainingTeams)
	fmt.Printf("Teams: %v\n", teams)
	fmt.Printf("nWeeks: %d\nweekNumber: %d\ndoubleDown: %v\n", nWeeks, *weekNumber, doubleDown)
	fmt.Printf("Probabilities:\n%v\n", probs)

	err = probs.KeepTeams(remainingTeams)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	err = probs.FilterWeeks(*weekNumber)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	fmt.Printf("Filtered Probabilities:\n%v\n", probs)

	// Here we go.

	// For permutations to work properly, these should be sorted
	sort.Strings(remainingTeams)

	// This is the best I can do
	var bestPerm orderperm

	// These are the results from my goroutines
	results := make(chan orderperm)

	if doubleDown {

		// Each dd team gets its own goroutine
		for _, ddt := range remainingTeams {
			_, ddw := maxFloat64(probs[ddt])

			teamsAfterDD, _ := remainingTeams.CopyWithoutTeam(ddt)
			probsAfterDD, _ := probs.CopyWithoutTeam(ddt)

			pPerThread := math.MaxInt32
			go permute(0, pPerThread, teamsAfterDD, probsAfterDD, ddt, ddw, results)

		}

	} else {

		// Divy up the permutations
		permutator, err := permutation.NewPerm(remainingTeams, nil)

		if err != nil {
			fmt.Print("unable to create permutation of remaining teams")
			os.Exit(-2)
		}

		nPermutations := permutator.Left()
		pPerThread := nPermutations / numCPU

		//wg := &sync.WaitGroup{}
		//wg.Add(numCPU)

		//bc, _ := multibar.New()
		//go bc.Listen()

		for i := 0; i < numCPU; i++ {
			//bc.MakeBar(pPerThread, fmt.Sprintf("permutations %d/%d", i+1, numCPU))
			go permute(i, pPerThread, remainingTeams, probs, "", -1, results)
		}
	}

	//wg.Wait()
	for i := 0; i < numCPU; i++ {
		perm := <-results
		if perm.ddweek >= 0 && perm.ddteam != "" {
			perm.prob *= probs[perm.ddteam][perm.ddweek]
		}
		if perm.prob > bestPerm.prob {
			bestPerm = perm
		}
	}

	fmt.Printf("Best: %v\n", bestPerm)

}

func parseFlags() (*csv.Reader, error) {
	flag.Parse()
	if *dataFile == "" {
		return nil, fmt.Errorf("data flag required")
	}

	csvFile, err := os.Open(*dataFile)
	if err != nil {
		return nil, err
	}
	//defer csvFile.Close()
	reader := csv.NewReader(csvFile)
	return reader, err
}

// Slices are passed by reference
func parseRow(row []string) (string, []float64, error) {
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

func parseSelection(teams []string, sel selection) []int {
	teamMap := make(map[string]int)
	for i, s := range teams {
		teamMap[s] = i
	}

	selected := make([]int, len(sel))
	for i, s := range sel {
		selected[i] = teamMap[s]
	}

	return selected
}

func maxFloat64(s []float64) (m float64, i int) {
	i = -1
	if len(s) > 0 {
		i = 0
		m = s[0]
	}
	for j, v := range s[1:] {
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

func permute(i int, pPerThread int, remainingTeams selection, probs probabilityMap, ddteam string, ddweek int, results chan orderperm) {
	thisSel := make(selection, len(remainingTeams))
	copy(thisSel, remainingTeams)

	// skip!
	for nSkip := 0; nSkip < pPerThread*i; nSkip++ {
		permutationNext(thisSel)
	}

	bestProb, _ := probs.TotalProb(thisSel)
	bestPerm := orderperm{bestProb, thisSel, ddteam, ddweek}
	//fmt.Printf("%d Selection %v Prob (%f)\n", i, bestSel, bestProb)

	for j := 0; j < pPerThread && permutationNext(thisSel); j++ {
		//bc.Bars[i].Update(j)
		totalProb, _ := probs.TotalProb(thisSel)
		if totalProb > bestPerm.prob {
			bestPerm.prob = totalProb
			copy(bestPerm.perm, thisSel)
			fmt.Printf("New best %v\n", bestPerm)
		}
	}

	results <- bestPerm
	//wg.Done()
}
