package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	//"github.com/fighterlyt/permutation"
	"os"
	"runtime"
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
	flag.Parse()

	csvFile, err := os.Open(*dataFile)
	if err != nil {
		fmt.Println("error: ", err.Error())
		return
	}
	defer csvFile.Close()
	reader := csv.NewReader(csvFile)
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("error: ", err.Error())
		return
	}
	if *doubleTeam != "" {
		switch dw := *doubleWeek; {
		case dw == 0:
			fmt.Printf("error: ddweek required\n")
			return
		case dw > uint(len(pastSelection)):
			fmt.Printf("error: invalid ddweek %d\n", *doubleWeek)
			return
		}
	}

	fmt.Printf("CPUs: %d\ndataFile: %s\ndoubleWeek: %d\ndoubleTeam: %s\npastSelection: %v\n\n",
		numCPU, *dataFile, *doubleWeek, *doubleTeam, pastSelection)
	fmt.Printf("Probabilities:\n%v\n", records)
}
