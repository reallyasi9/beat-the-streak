package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"sync"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"github.com/atgjack/prob"
	"github.com/reallyasi9/beat-the-streak/internal/bts"
	"google.golang.org/api/iterator"
	"gopkg.in/yaml.v2"
)

var scheduleFile = flag.String("schedule",
	"schedule.yaml",
	"YAML `file` containing schedule of all pick-your-pony contenders")
var nMC = flag.Int("n", 1000000, "`number` of Monte Carlo simulations to run for each team")
var hyperVariance = flag.Float64("var",
	4.723,
	"Assumed prior `standard deviation` of Sagarin ratings")

// ModelPerformance holds Firestore data for model performance, parsed from ThePredictionTracker.com
// TODO: Combine with bts-mc
type ModelPerformance struct {
	HomeBias          float64                `firestore:"bias"`
	StandardDeviation float64                `firestore:"std_dev"`
	Model             *firestore.DocumentRef `firestore:"model"`
}

// SagarinRating is a rating.  From Sagarin.  Stored in Firestore.  Simple.
// TODO: Combine with bts-mc
type SagarinRating struct {
	Rating float64                `firestore:"rating"`
	Team   *firestore.DocumentRef `firestore:"team"`
}

// TeamSchedule is a team's schedule in Firestore format
// TODO: Combine with bts-mc
type TeamSchedule struct {
	Team              *firestore.DocumentRef   `firestore:"team"`
	RelativeLocations []bts.RelativeLocation   `firestore:"locales"`
	Opponents         []*firestore.DocumentRef `firestore:"opponents"`
}

// YamlSchedule is a representation of a YAML schedule file
// TODO: Combine with scheduler
type YamlSchedule map[string][]string

func main() {
	flag.Parse()

	ctx := context.Background()

	conf := &firebase.Config{}
	app, err := firebase.NewApp(ctx, conf)
	if check(err) {
		return
	}

	fs, err := app.Firestore(ctx)
	if check(err) {
		return
	}
	defer fs.Close()

	makeTeamLookup(ctx, fs)

	// Get most recent season
	// TODO: Combine with bts-mc
	iter := fs.Collection("seasons").OrderBy("start", firestore.Desc).Limit(1).Documents(ctx)
	seasonDoc, err := iter.Next()
	if check(err) {
		return
	}
	iter.Stop()
	log.Printf("latest season discovered: %s", seasonDoc.Ref.ID)

	// Get most recent Sagarin Ratings proper
	// TODO: Combine with bts-mc
	iter = fs.Collection("sagarin").OrderBy("timestamp", firestore.Desc).Limit(1).Documents(ctx)
	sagRateDoc, err := iter.Next()
	if check(err) {
		return
	}
	iter.Stop()
	log.Printf("latest sagarin ratings discovered: %s", sagRateDoc.Ref.ID)

	// Get most recent predictions
	// TODO: Combine with bts-mc
	iter = fs.Collection("prediction_tracker").OrderBy("timestamp", firestore.Desc).Limit(1).Documents(ctx)
	predictionDoc, err := iter.Next()
	if check(err) {
		return
	}
	iter.Stop()
	log.Printf("latest prediction tracker discovered: %s", predictionDoc.Ref.ID)

	// Get Sagarin Rating performance
	// TODO: Combine with bts-mc
	iter = predictionDoc.Ref.Collection("model_performance").Where("system", "==", "Sagarin Points").Limit(1).Documents(ctx)
	sagDoc, err := iter.Next()
	if check(err) {
		return
	}
	iter.Stop()
	log.Printf("sagarin model performances discovered: %s", sagDoc.Ref.ID)

	var sagPerf ModelPerformance
	err = sagDoc.DataTo(&sagPerf)
	if check(err) {
		return
	}
	log.Printf("Sagarin Ratings performance: %v", sagPerf)

	homeAdvantage, err := sagRateDoc.DataAt("home_advantage_rating")
	if check(err) {
		return
	}
	log.Printf("Sagarin home advantage: %f", homeAdvantage)

	// Get teams while we are at it--this is more efficient than making multiple calls
	teamRefs := make([]*firestore.DocumentRef, 0)
	ratings := make([]float64, 0)

	iter = sagRateDoc.Ref.Collection("ratings").Documents(ctx)
	for {
		teamRatingDoc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if check(err) {
			return
		}

		var sr SagarinRating
		err = teamRatingDoc.DataTo(&sr)
		if check(err) {
			return
		}

		//log.Printf("Sagarin rating: %v", sr)
		teamRefs = append(teamRefs, sr.Team)
		ratings = append(ratings, sr.Rating)
	}
	iter.Stop()
	log.Printf("team ratings filled")

	teamDocs, err := fs.GetAll(ctx, teamRefs)
	if check(err) {
		return
	}

	teams := make([]bts.Team, len(teamDocs))
	for i, td := range teamDocs {
		var team bts.Team
		err := td.DataTo(&team)
		if check(err) {
			return
		}

		// log.Printf("team %v", team)
		teams[i] = team
	}

	// Build the probability model
	ratingsMap := make(map[bts.Team]float64)
	for i, t := range teams {
		ratingsMap[t] = ratings[i]
	}
	homeBias := sagPerf.HomeBias + homeAdvantage.(float64)
	closeBias := homeBias / 2.
	defaultModel := bts.NewGaussianSpreadModel(ratingsMap, sagPerf.StandardDeviation, homeBias, closeBias)

	log.Printf("Built model %v", defaultModel)

	yf, err := ioutil.ReadFile(*scheduleFile)
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
	var yamlSchedule YamlSchedule
	err = yaml.Unmarshal(yf, &yamlSchedule)
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
	log.Printf("read schedule: %v", yamlSchedule)

	byeWeekTeam := fs.Collection("teams").Doc("bye week")
	schedule := make(bts.Schedule)
	for t, opponents := range yamlSchedule {
		team1, exists := otherTeamLookup[t]
		if !exists {
			log.Fatalf(`team "%s" not found in teams`, t)
			return
		}

		opps := make([]*firestore.DocumentRef, len(opponents))
		locs := make([]int, len(opponents))
		for i, opp := range opponents {
			if opp == "" {
				opps[i] = byeWeekTeam
				locs[i] = bts.Neutral
				continue
			}

			loc, other := splitLocTeam(opp)
			team2, exists := otherTeamLookup[other]
			if !exists {
				log.Fatalf(`team "%s" not found in teams`, other)
				return
			}
			opps[i] = team2
			locs[i] = int(loc)
		}

		teamDoc, err := team1.Get(ctx)
		if check(err) {
			return
		}

		var team bts.Team
		err = teamDoc.DataTo(&team)
		if check(err) {
			return
		}

		schedule[team] = make([]*bts.Game, len(opps))
		for i, opponent := range opps {
			oppDoc, err := opponent.Get(ctx)
			if check(err) {
				return
			}

			var op bts.Team
			err = oppDoc.DataTo(&op)
			if check(err) {
				return
			}

			game := bts.NewGame(team, op, bts.RelativeLocation(locs[i]))
			schedule[team][i] = game
		}
	}

	log.Printf("Schedule built:\n%v", schedule)

	// // Loop through the teams
	results := make(chan teamResults, len(schedule))
	var wg sync.WaitGroup
	hv := *hyperVariance
	for team := range schedule {
		wg.Add(1)
		go func(t bts.Team) {
			wins := simulateWins(t, schedule, ratingsMap, homeBias, closeBias, sagPerf.StandardDeviation, hv)
			results <- teamResults{Team: t, WinProbabilities: wins}
			wg.Done()
		}(team)
	}
	wg.Wait()
	close(results)

	// Print the table header
	fmt.Print(" Team      Wins: ")
	for i := 0; i < schedule.NumWeeks(); i++ {
		fmt.Printf(" %5d ", i)
	}
	// if *week > 1 {
	// 	// Because of bye weeks, teams can have an additional win
	// 	fmt.Printf(" %5d ", bts.NGames-*week+1)
	// }
	fmt.Println()

	// Drain the results now
	for result := range results {
		t := result.Team
		fmt.Printf(" %15s ", t)
		for i := 0; i < schedule.NumWeeks(); i++ {
			fmt.Printf(" %5.3f ", result.WinProbabilities[i])
		}
		// if *week > 1 {
		// 	// Because of bye weeks, teams can have an additional win
		// 	fmt.Printf(" %5.3f ", result.WinProbabilities[bts.NGames-*week+1])
		// }
		fmt.Println()
	}

}

func check(err error) bool {
	if err != nil {
		log.Fatalln(err)
		return true
	}
	return false
}

var teamRefLookup = make(map[string]*firestore.DocumentRef)
var otherTeamLookup = make(map[string]*firestore.DocumentRef)

// TODO: Combine with bts-mc
func makeTeamLookup(ctx context.Context, fs *firestore.Client) error {
	teamIter := fs.Collection("teams").Documents(ctx)
	defer teamIter.Stop()
	for {
		teamDoc, err := teamIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		team4, err := teamDoc.DataAt("name_4")
		if err != nil {
			return err
		}

		teamRefLookup[team4.(string)] = teamDoc.Ref

		otherNames, err := teamDoc.DataAt("other_names")
		if err != nil {
			return err
		}

		for _, n := range otherNames.([]interface{}) {
			otherTeamLookup[n.(string)] = teamDoc.Ref
		}

	}
	return nil
}

// splitLocTeam splits a marked team name into a relative location and a team name.
// Note: this is relative to the schedule team, not the team given here.
// TODO: Combine with scheduler
func splitLocTeam(locTeam string) (bts.RelativeLocation, string) {
	if locTeam == "BYE" || locTeam == "" {
		return bts.Neutral, locTeam
	}

	switch locTeam[0] {
	case '@':
		return bts.Away, locTeam[1:]
	case '>':
		return bts.Far, locTeam[1:]
	case '<':
		return bts.Near, locTeam[1:]
	case '!':
		return bts.Neutral, locTeam[1:]
	default:
		return bts.Home, locTeam
	}
}

type teamResults struct {
	Team             bts.Team
	WinProbabilities []float64
}

// func worker(i int, jobs <-chan bts.Team, results chan<- teamResults, s bts.Schedule, r bts.Ratings, bias float64, std float64, hypervariance float64) {
// 	for t := range jobs {
// 		results <- teamResults{Team: t, WinProbabilities: simulateWins(t, s, r, bias, std, hypervariance)}
// 	}
// }

func simulateWins(team bts.Team, s bts.Schedule, r map[bts.Team]float64, hbias, cbias, std, hypervariance float64) []float64 {
	winHist := make([]int, len(s[team]))

	ratingNormal, err := prob.NewNormal(0, hypervariance)
	if err != nil {
		panic(err)
	}

	for i := 0; i < *nMC; i++ {
		// nudge ratings by a random amount
		myRatings := make(map[bts.Team]float64)
		for t, rating := range r {
			myRatings[t] = rating + ratingNormal.Random()
		}

		// calculate probabilities from nudged ratings
		model := bts.NewGaussianSpreadModel(myRatings, std, hbias, cbias)

		// Simulate wins from probabilities
		predictions := bts.MakePredictions(&s, model)
		wins := 0
		for week := 0; week < s.NumWeeks(); week++ {
			if rand.Float64() < predictions.GetProbability(team, week) {
				wins++
			}
		}

		// Count it
		winHist[wins]++
	}

	// Normalize win counts
	out := make([]float64, len(winHist))
	for i, win := range winHist {
		out[i] = float64(win) / float64(*nMC)
	}

	return out
}
