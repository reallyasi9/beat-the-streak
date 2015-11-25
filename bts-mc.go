package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	//"github.com/fighterlyt/permutation"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
)

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

var numCPU = runtime.GOMAXPROCS(0)
var dataFile = flag.String("data", "", "CSV file containing probabilities of win")
var doubleWeek = flag.Uint("ddweek", 0, "double-down week (1-indexed)")
var doubleTeam = flag.String("ddteam", "", "double-down team")
var pastSelection selection

func init() {
	flag.Var(&pastSelection, "selection", "comma-separated list of selected teams, in order (excluding double-down)")
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

	fmt.Printf("CPUs: %d\ndataFile: %s\ndoubleWeek: %d\ndoubleTeam: %s\npastSelection: %v\n\n",
		numCPU, *dataFile, *doubleWeek, *doubleTeam, pastSelection)
	fmt.Printf("Teams:%v\n", teams)
	fmt.Printf("Probabilities:\n%v\n", probs)

	pastIndices := parseSelection(teams, pastSelection)
	// Delete rows from the probability table
	for _, i := range pastIndices {
		probs = append(probs[:i], probs[i+1:]...)
	}

	fmt.Printf("Cleaned Probabilities:\n%v\n", probs)
}

func parseFlags() (*csv.Reader, error) {
	flag.Parse()

	csvFile, err := os.Open(*dataFile)
	if err != nil {
		return nil, err
	}
	//defer csvFile.Close()
	reader := csv.NewReader(csvFile)
	if *doubleTeam != "" {
		switch dw := *doubleWeek; {
		case dw == 0:
			return reader, errors.New("ddweek required")
		case dw > uint(len(pastSelection)):
			fmt.Printf("error: invalid ddweek %d\n", *doubleWeek)
			return reader, fmt.Errorf("invalid ddweek %d\n", *doubleWeek)
		}
	}
	return reader, err
}

// Slices are passed by reference
func parseRow(row []string) (string, []float64, error) {
	var err error
	team := row[0]
	probs := make([]float64, len(row)-1)
	for i, rec := range row[1:] {
		if rec == "#N/A" {
			continue
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
	return probs[selections[0]][0] * totalProb(probs[:][1:], selections[1:])
}
