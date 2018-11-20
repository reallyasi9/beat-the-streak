package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"sort"
	"strings"

	"runtime"

	"../bts"
	"github.com/atgjack/prob"

	yaml "gopkg.in/yaml.v2"
)

type nameMap map[string]string

func makeNameMap(filename string) (nameMap, error) {
	nameYaml, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	nm := make(nameMap)
	err = yaml.Unmarshal(nameYaml, nm)
	if err != nil {
		return nil, err
	}

	return nm, nil
}

type superdogGame struct {
	GameModel bts.GameModel
	Dog       string
	Spread    float64
	Prob      float64
}

type byDog []superdogGame

func (s byDog) Len() int {
	return len(s)
}
func (s byDog) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byDog) Less(i, j int) bool {
	return s[i].Dog < s[j].Dog
}

var numCPU = runtime.GOMAXPROCS(0)
var linesURL = flag.String("lines",
	"http://www.thepredictiontracker.com/ncaapredictions.csv",
	"`URL` of lines CSV file for calculating probabilities of win")
var model = flag.String("model",
	"line",
	"`MODEL` name for looking up performance and win probabilities (typically begins with 'line')")
var performanceURL = flag.String("performance",
	"http://www.thepredictiontracker.com/ncaaresults.php",
	"`URL` of model performances for calculating probabilities of win")
var namesFile = flag.String("names",
	"modelnames.yaml",
	"`FILE` containing name lookup between prediction and performance model names")

func main() {
	flag.Parse()

	lines, err := bts.MakeLines(*linesURL)
	if err != nil {
		panic(err)
	}
	log.Printf("Downloaded lines %v", lines)

	names, err := makeNameMap(*namesFile)
	if err != nil {
		panic(err)
	}
	log.Printf("Loaded names map %v", names)

	verbModel, ok := names[*model]
	if !ok {
		panic(fmt.Errorf("unable to parse model name \"%s\"", *model))
	}
	log.Printf("Parsed verbose model name \"%s\"", verbModel)

	bias, stdDev, err := bts.ScrapeParameters(*performanceURL, verbModel)
	if err != nil {
		panic(err)
	}
	log.Printf("Scraped bias %f, standard dev %f", bias, stdDev)

	normal := prob.Normal{Mu: 0, Sigma: stdDev}

	// Make a sorted list of games and extract the length of the the longest team name
	gameModels := make(byDog, 0)
	maxNameLen := 0
	for gm, line := range lines {
		if gm.Model != *model {
			continue
		}
		p := normal.Cdf(line + bias)

		var ud string
		var udp float64
		if p < 0.5 {
			ud = gm.Game.HomeTeam
			udp = p
		} else {
			ud = gm.Game.AwayTeam
			udp = 1 - p
		}

		gameModels = append(gameModels, superdogGame{GameModel: gm, Dog: ud, Prob: udp, Spread: line + bias})

		if len(gm.Game.HomeTeam) > maxNameLen {
			maxNameLen = len(gm.Game.HomeTeam)
		}
		if len(gm.Game.AwayTeam) > maxNameLen {
			maxNameLen = len(gm.Game.AwayTeam)
		}
	}
	sort.Sort(gameModels)

	// Pretty print a table header
	tableWidth := maxNameLen*3 + 6 + 5 + 4*3 + 2*7
	fmt.Printf("%[1]*[2]s  %[1]*[3]s  %[1]*[4]s  %-6[5]s  %-5[6]s  %-4[7]s  %-4[8]s  %-4[9]s\n",
		-maxNameLen, "Home", "Away", "Dog", "spread", "prob", "@3", "@4", "@5")
	fmt.Printf("%s\n", strings.Repeat("-", tableWidth))

	for _, gm := range gameModels {
		ht := gm.GameModel.Game.HomeTeam
		at := gm.GameModel.Game.AwayTeam
		ud := gm.Dog
		udp := gm.Prob
		spread := gm.Spread
		fmt.Printf("%[1]*[2]s  %[1]*[3]s  %[1]*[4]s  ", -maxNameLen, ht, at, ud)
		fmt.Printf("%#+6.2f  %#5.3f  %#4.2f  %#4.2f  %#4.2f\n", spread, udp, udp*3, udp*4, udp*5)
	}

}
