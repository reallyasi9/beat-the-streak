package bts

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/atgjack/prob"
)

// PredictionModel describes an object that can predict the probability a given team will defeat another team, or the point spread if those teams were to play.
type PredictionModel interface {
	MostLikelyOutcome(*Game) (team Team, prob float64, spread float64)
	Predict(*Game) (prob float64, spread float64)
}

// GaussianSpreadModel implements PredictionModel and uses a normal distribution based on spreads to calculate win probabilities.
// The spread is determined by a team rating and where the game is being played (to account for bias).
type GaussianSpreadModel struct {
	dist      prob.Normal
	homeBias  float64
	closeBias float64
	ratings   map[Team]float64
}

// NewGaussianSpreadModel makes a model.
func NewGaussianSpreadModel(ratings map[Team]float64, stdDev, homeBias, closeBias float64) *GaussianSpreadModel {
	return &GaussianSpreadModel{ratings: ratings, dist: prob.Normal{Mu: 0, Sigma: stdDev}, homeBias: homeBias, closeBias: closeBias}
}

// Predict returns the probability and spread for team1.
func (m GaussianSpreadModel) Predict(game *Game) (float64, float64) {
	if game.Team(0) == BYE || game.Team(1) == BYE {
		return 0., 0.
	}
	if game.Team(0) == NONE || game.Team(1) == NONE {
		return 1., 0.
	}
	spread := m.spread(game)
	prob := m.dist.Cdf(spread)

	return prob, spread
}

// MostLikelyOutcome returns the most likely team to win a given game, the probability of win, and the predicted spread.
func (m GaussianSpreadModel) MostLikelyOutcome(game *Game) (Team, float64, float64) {
	if game.Team(0) == BYE || game.Team(1) == BYE {
		return BYE, 0., 0.
	}
	if game.Team(0) == NONE || game.Team(1) == NONE {
		return NONE, 1., 0.
	}
	prob, spread := m.Predict(game)
	if spread < 0 {
		return game.Team(1), 1 - prob, -spread
	}
	return game.Team(0), prob, spread
}

func (m GaussianSpreadModel) spread(game *Game) float64 {
	diff := m.ratings[game.Team(0)] - m.ratings[game.Team(1)]
	switch game.LocationRelativeToTeam(0) {
	case Home:
		diff += m.homeBias
	case Near:
		diff += m.closeBias
	case Far:
		diff -= m.closeBias
	case Away:
		diff -= m.homeBias
	}
	return diff
}

// MakeGaussianSpreadModel makes a spread model by parsing Sagarin ratings and performance to date metrics.
func MakeGaussianSpreadModel(ratingsURL, performanceURL, modelName string) (*GaussianSpreadModel, error) {
	body, err := getURLBody(ratingsURL)
	if err != nil {
		return nil, err
	}

	edgeRegex := regexp.MustCompile("HOME ADVANTAGE=.*?\\[<font color=\"#0000ff\">\\s*([\\-0-9.]+)")
	edgeMatch := edgeRegex.FindStringSubmatch(string(body))
	if edgeMatch == nil {
		return nil, fmt.Errorf("unable to parse home advantage from %s", ratingsURL)
	}
	edge, err := strconv.ParseFloat(edgeMatch[1], 64)
	if err != nil {
		return nil, err
	}

	ratingsRegex := regexp.MustCompile("<font color=\"#000000\">\\s+\\d+\\s+(.*?)\\s+[A]+\\s*=<.*?<font color=\"#0000ff\">\\s*([\\-0-9.]+)")
	ratingsStr := ratingsRegex.FindAllStringSubmatch(string(body), -1)
	if ratingsStr == nil {
		return nil, fmt.Errorf("unable to parse any ratings from %s", ratingsURL)
	}

	ratings := make(map[Team]float64)
	for _, matches := range ratingsStr {
		rval, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			return nil, err
		}
		ratings[Team(matches[1])] = rval
	}

	bias, std, err := scrapeParameters(performanceURL, modelName)
	if err != nil {
		return nil, err
	}

	homeBias := edge + bias
	closeBias := (edge + bias) / 2.

	return NewGaussianSpreadModel(ratings, std, homeBias, closeBias), nil
}

func scrapeParameters(url string, modelName string) (float64, float64, error) {
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

func (m GaussianSpreadModel) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("home bias: %f; close bias: %f;\n", m.homeBias, m.closeBias))
	for t, r := range m.ratings {
		b.WriteString(fmt.Sprintf("%s: %f\n", t, r))
	}
	return b.String()
}
