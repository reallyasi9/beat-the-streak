package main

import (
	"flag"
	"fmt"
	"log"

	"../bts"

	"runtime"
)

var numCPU = runtime.GOMAXPROCS(0)
var outcomesFile = flag.String("outcomes",
	"outcomes.yaml",
	"YAML `file` containing outcomes for B1G schedule (1 = win, 0 = loss)")

func main() {
	flag.Parse()

	oc, err := makeOutcomes(*outcomesFile)
	if err != nil {
		panic(err)
	}
	log.Printf("Constructed outcomes %v", oc)

	// Here we go.
	results := worker(oc)

	// Drain the results now
	var nSolutions uint64
	for res := range results {

		nSolutions++
		for _, tn := range res.Teams {
			fmt.Printf("%s,", tn)
		}
		if res.DD != nil {
			fmt.Printf("%d,%s", res.DD.Week, res.DD.Team)
		} else {
			fmt.Print("0,")
		}
		fmt.Println()
	}

}

func worker(oc *outcomes) chan bts.Streak {
	out := make(chan bts.Streak)
	go oc.countPaths(out)
	return out
}
