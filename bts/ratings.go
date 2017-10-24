package bts

import (
	"fmt"
	"math"
	"regexp"
	"strconv"

	"github.com/atgjack/prob"
)

type Ratings map[Team]float64

func (r Ratings) MakeProbabilities(s Schedule, bias, stdDev float64) (Probabilities, Spreads, error) {
	normal := prob.Normal{Mu: 0, Sigma: stdDev}

	p := make(Probabilities)
	spr := make(Spreads)

	for team1, sched := range s {
		p[team1] = make([]float64, 13)
		spr[team1] = make([]float64, 13)
		rating1, ok := r[team1]
		if !ok {
			return nil, nil, fmt.Errorf("team %s not in ratings", team1)
		}
		for i, team2 := range sched {
			if team2 == "" {
				p[team1][i] = 0.
				spr[team1][i] = 0.
				continue
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
				return nil, nil, fmt.Errorf("team %s (opponent of %s in week %d) not in ratings", team2, team1, i+1)
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
			rawp := normal.Cdf(spread)
			capp := rawp
			spr[team1][i] = math.Abs(spread)
			p[team1][i] = capp
		}
	}

	return p, spr, nil
}

func MakeRatings(url string) (Ratings, error) {
	body, err := getURLBody(url)
	if err != nil {
		return nil, err
	}

	ratingsRegex := regexp.MustCompile("<font color=\"#000000\">\\s+\\d+\\s+(.*?)\\s+[A]+\\s*=<.*?<font color=\"#0000ff\">\\s*([\\-0-9.]+)")
	ratingsStr := ratingsRegex.FindAllStringSubmatch(string(body), -1)
	if ratingsStr == nil {
		return nil, fmt.Errorf("unable to parse any ratings from %s", url)
	}

	r := make(Ratings)
	for _, matches := range ratingsStr {
		rval, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			return nil, err
		}
		r[Team(matches[1])] = rval
	}

	return r, nil
}

func ScrapeParameters(url string, modelName string) (float64, float64, error) {
	body, err := getURLBody(url)
	if err != nil {
		return 0., 0., err
	}

	perfRegex := regexp.MustCompile(fmt.Sprintf("%s</font>.*?>[\\-0-9.]+<.*?>[\\-0-9.]+<.*?>[\\-0-9.]+<.*?>([\\-0-9.]+)<.*?>([\\-0-9.]+)<", modelName))
	perfStr := perfRegex.FindSubmatch(body)
	if perfStr == nil {
		return 0., 0., fmt.Errorf("unable to parse bais and mean squared error for model %s from %s", modelName, url)
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
