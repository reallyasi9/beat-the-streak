package main

import (
	"fmt"
	"io/ioutil"
	"sync"

	"../bts"

	yaml "gopkg.in/yaml.v2"
)

type outcomes map[string][]bool

func makeOutcomes(fn string) (*outcomes, error) {
	outcomeYaml, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	o := make(outcomes)
	err = yaml.Unmarshal(outcomeYaml, o)
	if err != nil {
		return nil, err
	}

	for k, v := range o {
		if len(v) != 13 {
			return nil, fmt.Errorf("outcomes for team %s incorrect: expected 13, got %d", k, len(v))
		}
	}

	return &o, nil
}

func (oc *outcomes) countPaths(out chan<- bts.Streak) {
	defer close(out)

	jobs := make(chan bts.TeamList, 100)
	results := make(chan bts.TeamList, 100)
	wg := new(sync.WaitGroup)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go cpWorker(jobs, results, oc, wg)
	}

	wg2 := new(sync.WaitGroup)
	for i := 0; i < 100; i++ {
		wg2.Add(1)
		go streakWorker(results, out, oc, wg)
	}

	for ddt := range *oc {
		tl := make(bts.TeamList, 0)
		for t := range *oc {
			if t == ddt {
				continue
			}
			tl = append(tl, bts.Team(t))
		}

		jobs <- tl
	}

	wg.Wait()

	close(results)

	wg2.Wait()
}

func (oc *outcomes) validate(tl bts.TeamList) bool {
	for week, team := range tl {
		if !(*oc)[string(team)][week] {
			return false
		}
	}
	return true
}

func cpWorker(jobs chan bts.TeamList, results chan<- bts.TeamList, oc *outcomes, wg *sync.WaitGroup) {
	defer wg.Done()
	for j := range jobs {
		c := make(chan bts.TeamList, 100)
		go bts.TeamPermute(j, c)
		for tl := range c {
			if oc.validate(tl) {
				results <- tl
			}
		}
	}
}

func streakWorker(jobs chan bts.TeamList, results chan<- bts.Streak, oc *outcomes, wg *sync.WaitGroup) {
	defer wg.Done()

	for tl := range jobs {
		found := make(map[string]bool)
		for _, team := range tl {
			found[string(team)] = true
		}

		for ddt, v := range *oc {
			if _, ok := found[ddt]; ok {
				continue
			}

			for week, wl := range v {
				if !wl {
					continue
				}
				dd := &bts.DoubleDown{Team: bts.Team(ddt), Week: week}
				s := bts.Streak{Teams: tl, DD: dd}
				results <- s
			}
		}
	}
}
