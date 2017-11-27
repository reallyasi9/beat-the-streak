package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"runtime"

	"../bts"
	yaml "gopkg.in/yaml.v2"
)

var numCPU = runtime.GOMAXPROCS(0)
var outcomesFile = flag.String("outcomes",
	"outcomes.yaml",
	"YAML `file` containing outcomes for B1G schedule (1 = win, 0 = loss)")
var scheduleFile = flag.String("schedule",
	"schedule.yaml",
	"YAML `file` containing B1G schedule")

func main() {
	flag.Parse()

	schedule, err := bts.MakeSchedule(*scheduleFile)
	if err != nil {
		panic(err)
	}
	log.Printf("Made schedule %v", schedule)

	probs, err := makeOutcomes(*outcomesFile, schedule)
	if err != nil {
		panic(err)
	}
	log.Printf("Constructed outcomes %v", probs)

	// The only "Player" in this context is a single player with all picks remaining
	player := make(bts.Player, len(schedule))
	i := 0
	for tn := range schedule {
		player[i] = string(tn)
		i++
	}

	log.Printf("Made player %v", player)

	// Here we go.
	// Loop through the (one) unique users
	results := make(chan bts.StreaksByProb)
	jobs := make(chan bts.Player)
	go worker(jobs, results, *probs)
	jobs <- player
	close(jobs)

	// Drain the results now
	result := <-results
	nSolutions := 0
	for _, res := range result {
		if res.Streak == nil || res.Prob == 0 {
			continue
		}
		nSolutions++
		for _, tn := range res.Streak.Teams {
			fmt.Printf("%s,", tn)
		}
		if res.Streak.DD != nil {
			fmt.Printf("%d,%s", res.Streak.DD.Week, res.Streak.DD.Team)
		} else {
			fmt.Print("0,")
		}
		fmt.Println()
	}

	close(results)

}

func worker(jobs <-chan bts.Player, results chan<- bts.StreaksByProb, probs bts.Probabilities) {
	for p := range jobs {
		results <- p.BestStreaks(probs, true, 100000000)
	}
}

func makeOutcomes(fn string, sched bts.Schedule) (*bts.Probabilities, error) {
	outcomeYaml, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	p := make(bts.Probabilities)
	err = yaml.Unmarshal(outcomeYaml, p)
	if err != nil {
		return nil, err
	}

	for k, v := range p {
		if len(v) != 13 {
			return nil, fmt.Errorf("outcomes for team %s incorrect: expected %d, got %d", k, 13, len(v))
		}
	}

	return &p, nil
}
