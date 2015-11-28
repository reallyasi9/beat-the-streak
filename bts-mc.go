package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"github.com/cespare/permute"
	"github.com/sethgrid/multibar"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	//"sync"
)

// Convenience type, so I can parse a list of strings from the command line
type selection []string

// String method, part of the flag.Value interface
func (s *selection) String() string {
	return fmt.Sprint(*s)
}

// Set method, part of the flag.Value interface
func (s *selection) Set(value string) error {
	if len(*s) > 0 {
		return errors.New("selection flag already set")
	}
	for _, sel := range strings.Split(value, ",") {
		*s = append(*s, sel)
	}
	return nil
}

// Convenience type, so I can return both a probability and a selection from a goroutine
type permutation struct {
	prob   float64
	perm   []int
	ddprob float64
	ddweek int
}

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
		return
	}

	// Throw away the first row (headers)
	_, err = reader.Read()
	if err != nil {
		fmt.Printf("error reading first row : %s\n", err)
		return
	}

	// Parse remaining data and store it
	var teams []string
	var probs [][]float64
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("error reading data : %s\n", err)
			return
		}

		if len(row) == 0 {
			break
		}

		team, prob, err := parseRow(row)
		if err != nil {
			fmt.Printf("error parsing row : %s\n", err)
			return
		}

		teams = append(teams, team)
		probs = append(probs, prob)
	}

	if len(teams) != len(probs) {
		fmt.Printf("error parsing data : %d teams != %d rows\n", len(teams), len(probs))
		return
	}

	nWeeks := int(0)
	for i, row := range probs {
		if nWeeks == 0 {
			nWeeks = len(row)
			continue
		}
		if len(row) != nWeeks {
			fmt.Printf("error parsing data : weeks in row %d (%d) != weeks in row 1 (%d)\n", i, len(row), nWeeks)
			return
		}
	}

	// Can't be predicting past the end of the season
	if *weekNumber > nWeeks {
		fmt.Printf("error parsing week : only %d weeks in data, week %d requested\n", nWeeks, *weekNumber)
		return
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
			return
		}
		if len(remainingTeams) < nWeeks-*weekNumber+1 {
			fmt.Printf("error parsing remaining : not enough teams remaining (%d) to fill remaining weeks (%d)\n", len(remainingTeams), nWeeks-*weekNumber+1)
			return
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

	remainingIndices := parseSelection(teams, remainingTeams)
	// Filter rows from the probability table
	filteredProbs := probs[:0]
	for _, i := range remainingIndices {
		filteredProbs = append(filteredProbs, probs[i])
	}

	fmt.Printf("Filtered Probabilities:\n%v\n", filteredProbs)

	progressBars, _ := multibar.New()

	indices := make([]int, len(filteredProbs))
	for i := 0; i < len(filteredProbs); i++ {
		indices[i] = i
	}

	remainingN, err := factorial(int64(len(remainingIndices) - 1))
	if err != nil {
		fmt.Println(err)
		return
	}

	if doubleDown {
		progressBars.Println("Double-Down Progress")

		for ddt := range remainingIndices {
			progressBars.MakeBar(int(remainingN), teams[ddt])
		}

		results := make(chan permutation, len(remainingIndices))
		for _, ddi := range remainingIndices {
			ddprob, ddweek := maxFloat64(probs[ddi])

			go func(ddi int, remainingIndices []int) {

				// Cut from remaining indices
				var theseIndices []int
				copy(theseIndices, remainingIndices)
				theseIndices = append(theseIndices[:ddi], theseIndices[ddi+1:]...)
				// Filter rows from the probability table
				theseProbs := filteredProbs[:0]
				for _, i := range theseIndices {
					theseProbs = append(theseProbs, filteredProbs[i])
				}

				idx := make([]int, len(theseIndices))
				copy(idx, theseIndices)
				bestProb := 0.
				bestSel := make([]int, len(theseProbs))
				count := int64(0)
				p := permute.Ints(idx)
				for p.Permute() {
					thisProb := totalProb(theseProbs, idx)
					if thisProb > bestProb {
						bestProb = thisProb
						copy(bestSel, idx)
					}
					count++
					progressBars.Bars[ddi].Update(int(count))
				}
				results <- permutation{bestProb, bestSel, ddprob, ddweek}
			}(ddi, remainingIndices)
		}

		best := <-results
		bestdd := 0
		for i := 1; i < len(remainingIndices); i++ {
			tmp := <-results
			if tmp.prob*tmp.ddprob > best.prob*best.ddprob {
				best = tmp
				bestdd = i
			}
		}

		fmt.Printf("Best: %v @ %f (with %d on %d)\n", best.perm, best.prob, bestdd, best.ddweek)

	} else {
		updateFunction := progressBars.MakeBar(int(remainingN), "Permuting")
		bestProb := 0.
		bestSel := make([]int, len(filteredProbs))
		count := int64(0)
		p := permute.Ints(indices)
		for p.Permute() {
			thisProb := totalProb(filteredProbs, remainingIndices)
			if thisProb > bestProb {
				bestProb = thisProb
				copy(bestSel, indices)
			}
			count++
			updateFunction(int(count))
		}

		fmt.Printf("Best: %v @ %f\n", bestSel, bestProb)
	}

}

func parseFlags() (*csv.Reader, error) {
	flag.Parse()
	if *dataFile == "" {
		return nil, errors.New("data flag required")
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

func totalProb(probs [][]float64, selections []int) float64 {
	if len(selections) == 1 {
		return probs[selections[0]][0]
	}
	p := probs[selections[0]][0]
	if p == 0 {
		return 0
	}
	return p * totalProb(probs[:][1:], selections[1:])
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

func factorial(v int64) (f int64, err error) {
	if v < 0 {
		err = errors.New("argument must be positive")
		return 0, err
	}
	if v <= 1 {
		return 1, nil
	}
	v1, _ := factorial(v - 1)
	return v * v1, nil
}
