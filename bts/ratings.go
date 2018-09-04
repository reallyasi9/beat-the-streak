package bts

import (
	"fmt"
	"math"
	"regexp"
	"strconv"

	"github.com/atgjack/prob"
)

type Ratings map[Team]float64

func (r Ratings) TrueSpread(team1, team2 Team, bias float64) (float64, error) {
	if team2 == "" {
		return 0., nil
	}

	rating1, ok := r[team1]
	if !ok {
		return 0., fmt.Errorf("team \"%s\" not in ratings", team1)
	}

	t2home := team2[0] == '@'
	t2close := team2[0] == '>'
	t1close := team2[0] == '<'
	neutral := team2[0] == '!'
	if t2home || t2close || t1close || neutral {
		team2 = team2[1:]
	}

	rating2, ok := r[team2]
	if !ok {
		return 0., fmt.Errorf("team \"%s\" not in ratings", team2)
	}

	spread := rating1 - rating2
	if t2home {
		spread -= bias
	} else if t2close {
		spread -= bias / 2
	} else if t1close {
		spread += bias / 2
	} else if !neutral {
		spread += bias
	}

	return spread, nil
}

func (r Ratings) MakeProbabilities(s Schedule, bias, stdDev float64) (Probabilities, Spreads, error) {
	normal := prob.Normal{Mu: 0, Sigma: stdDev}

	p := make(Probabilities)
	spr := make(Spreads)

	for team1, sched := range s {
		p[team1] = make([]float64, 13)
		spr[team1] = make([]float64, 13)
		for i, team2 := range sched {
			if team2 == "" {
				p[team1][i] = 0.
				spr[team1][i] = 0.
				continue
			}
			spread, err := r.TrueSpread(team1, team2, bias)
			if err != nil {
				return nil, nil, err
			}
			rawp := normal.Cdf(spread)
			spr[team1][i] = spread
			p[team1][i] = rawp
		}
	}

	return p, spr, nil
}

func MakeRatings(url string) (Ratings, float64, error) {
	body, err := getURLBody(url)
	if err != nil {
		return nil, 0., err
	}

	edgeRegex := regexp.MustCompile("HOME ADVANTAGE=.*?\\[<font color=\"#0000ff\">\\s*([\\-0-9.]+)")
	edgeMatch := edgeRegex.FindStringSubmatch(string(body))
	if edgeMatch == nil {
		return nil, 0., fmt.Errorf("unable to parse home advantage from %s", url)
	}
	edge, err := strconv.ParseFloat(edgeMatch[1], 64)
	if err != nil {
		return nil, 0., err
	}

	ratingsRegex := regexp.MustCompile("<font color=\"#000000\">\\s+\\d+\\s+(.*?)\\s+[A]+\\s*=<.*?<font color=\"#0000ff\">\\s*([\\-0-9.]+)")
	ratingsStr := ratingsRegex.FindAllStringSubmatch(string(body), -1)
	if ratingsStr == nil {
		return nil, 0., fmt.Errorf("unable to parse any ratings from %s", url)
	}

	r := make(Ratings)
	for _, matches := range ratingsStr {
		rval, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			return nil, 0., err
		}
		r[Team(matches[1])] = rval
	}

	return r, edge, nil
}

func ScrapeParameters(url string, modelName string) (float64, float64, error) {
	body, err := getURLBody(url)
	if err != nil {
		return 0., 0., err
	}

	perfRegex := regexp.MustCompile(fmt.Sprintf("%s</font>.*?<font size=2>[\\-0-9.]*</font>.*?<font size=2>[\\-0-9.]*</font>.*?<font size=2>[\\-0-9.]*</font>.*?<font size=2>([\\-0-9.]+)</font>.*?<font size=2>([\\-0-9.]+)</font>", regexp.QuoteMeta(modelName)))
	perfStr := perfRegex.FindSubmatch(body)
	if perfStr == nil {
		return 0., 0., fmt.Errorf("unable to parse bais and mean squared error for model \"%s\" from \"%s\"", modelName, url)
	}
	bias, err := strconv.ParseFloat(string(perfStr[1]), 64)
	if err != nil {
		return 0., 0., err
	}
	mse, err := strconv.ParseFloat(string(perfStr[2]), 64)
	if err != nil {
		return 0., 0., err
	}
	std := math.Sqrt(mse - bias*bias)
	return bias, std, nil
}
