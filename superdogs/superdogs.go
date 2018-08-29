package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"

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

	fmt.Println(" Home         Away         Underdog     Spread       p(superdog)  Value@3      Value@4      Value@5")
	fmt.Println(" --------------------------------------------------------------------------------------------------")
	for gm, line := range lines {
		if gm.Model != *model {
			continue
		}
		p := normal.Cdf(line + bias)

		ht := gm.Game.HomeTeam
		at := gm.Game.AwayTeam
		if len(ht) > 12 {
			ht = ht[:12]
		}
		if len(at) > 12 {
			at = at[:12]
		}
		fmt.Printf(" %12s %12s", ht, at)
		var ud string
		var udp float64
		if p < 0.5 {
			ud = ht
			udp = p
		} else {
			ud = at
			udp = 1 - p
		}

		fmt.Printf(" %12s %12.2f %12.2f %12.2f %12.2f %12.2f\n", ud, line+bias, udp, udp*3, udp*4, udp*5)
	}

	// probs, spreads, err := ratings.MakeProbabilities(schedule, bias, stdDev)
	// if err != nil {
	// 	panic(err)
	// }
	// log.Printf("Made probabilities\n%s", probs)
	// log.Printf("Made spreads\n%s", spreads)

}
