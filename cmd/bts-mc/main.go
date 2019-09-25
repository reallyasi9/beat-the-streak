package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"github.com/reallyasi9/beat-the-streak/internal/bts"
	"google.golang.org/api/iterator"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

var projectID = flag.String("project", "", "Google Cloud `project` to use")
var weekNumber = flag.Int("week", -1, "Week `number` (starting at 0)")
var nTop = flag.Int("n", 5, "`number` of top probabilities to report for each player to check for better spreads")

var startTime = time.Now()

// ModelPerformance holds Firestore data for model performance, parsed from ThePredictionTracker.com
type ModelPerformance struct {
	HomeBias          float64 `firestore:"bias"`
	StandardDeviation float64 `firestore:"std_dev"`
	System            string  `firestore:"system"`
}

// SagarinRating is a rating.  From Sagarin.  Stored in Firestore.  Simple.
type SagarinRating struct {
	Rating  float64                `firestore:"rating"`
	TeamRef *firestore.DocumentRef `firestore:"team_id"`
}

// TeamSchedule is a team's schedule in Firestore format
type TeamSchedule struct {
	TeamRef           *firestore.DocumentRef   `firestore:"team"`
	RelativeLocations []bts.RelativeLocation   `firestore:"locales"`
	Opponents         []*firestore.DocumentRef `firestore:"opponents"`
}

// PickerStreak is a picker's latest streak status, stored in the firestore database.
type PickerStreak struct {
	PickTypes      []int                    `firestore:"pick_types_remaining"`
	Picker         *firestore.DocumentRef   `firestore:"picker"`
	RemainingTeams []*firestore.DocumentRef `firestore:"remaining"`
}

// Picker is a picker.  Huh.
type Picker struct {
	Name string `firestore:"name_luke"`
}

func main() {
	ctx := context.Background()
	flag.Parse()

	log.Printf("Beating the streak on project %s, week number %d", *projectID, *weekNumber)

	conf := &firebase.Config{ProjectID: *projectID}
	app, err := firebase.NewApp(ctx, conf)

	check(err)

	fs, err := app.Firestore(ctx)
	check(err)
	defer fs.Close()

	// Get most recent predictions
	iter := fs.Collection("prediction_tracker").OrderBy("timestamp", firestore.Desc).Limit(1).Documents(ctx)
	predictionDoc, err := iter.Next()
	check(err)
	iter.Stop()

	// Get Sagarin Rating performance
	iter = predictionDoc.Ref.Collection("modelperformance").Where("system", "==", "Sagarin Ratings").Limit(1).Documents(ctx)
	sagDoc, err := iter.Next()
	check(err)
	iter.Stop()

	var sagPerf ModelPerformance
	sagDoc.DataTo(&sagPerf)
	log.Printf("Sagarin Ratings performance: %v", sagPerf)

	// Get most recent Sagarin Ratings proper
	iter = fs.Collection("sagarin").OrderBy("timestamp", firestore.Desc).Limit(1).Documents(ctx)
	sagRateDoc, err := iter.Next()
	check(err)
	iter.Stop()

	homeAdvantage, err := sagRateDoc.DataAt("home_advantage")
	check(err)
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
		check(err)
		var sr SagarinRating
		teamRatingDoc.DataTo(&sr)

		log.Printf("Sagarin rating: %v", sr)
		teamRefs = append(teamRefs, sr.TeamRef)
		ratings = append(ratings, sr.Rating)
	}
	iter.Stop()

	teamDocs, err := fs.GetAll(ctx, teamRefs)
	check(err)

	teams := make([]bts.Team, len(teamDocs))
	for i, td := range teamDocs {
		var team bts.Team
		err := td.DataTo(&team)
		check(err)
		log.Printf("team %v", team)
		teams[i] = team
	}

	// Build the probability model
	ratingsMap := make(map[bts.Team]float64)
	for i, t := range teams {
		ratingsMap[t] = ratings[i]
	}
	homeBias := sagPerf.HomeBias + homeAdvantage.(float64)
	closeBias := homeBias / 2.
	model := bts.NewGaussianSpreadModel(ratingsMap, sagPerf.StandardDeviation, homeBias, closeBias)

	log.Printf("Built model %v", model)

	// Get most recent season
	iter = fs.Collection("seasons").OrderBy("start", firestore.Desc).Limit(1).Documents(ctx)
	seasonDoc, err := iter.Next()
	check(err)
	iter.Stop()

	// log.Printf("Most recent season %v", seasonDoc)

	// Get schedule from most recent season
	iter = fs.Collection("schedules").Where("season", "==", seasonDoc.Ref).Limit(1).Documents(ctx)
	scheduleDoc, err := iter.Next()
	check(err)
	iter.Stop()

	// log.Printf("Most recent schedule %v", scheduleDoc)

	// Convert into schedule for predictions
	schedule := make(bts.Schedule)
	iter = scheduleDoc.Ref.Collection("teams").Documents(ctx)
	for {
		teamSchedule, err := iter.Next()
		if err == iterator.Done {
			break
		}
		check(err)

		var ts TeamSchedule
		err = teamSchedule.DataTo(&ts)
		check(err)

		opponentDocs, err := fs.GetAll(ctx, ts.Opponents)
		check(err)

		teamDoc, err := ts.TeamRef.Get(ctx)
		check(err)

		var team bts.Team
		err = teamDoc.DataTo(&team)
		check(err)

		schedule[team] = make([]*bts.Game, len(opponentDocs))
		for i, opponent := range opponentDocs {
			var op bts.Team
			err = opponent.DataTo(&op)
			check(err)

			game := bts.NewGame(team, op, ts.RelativeLocations[i])
			log.Printf("game loaded %v", game)
			schedule[team][i] = game

		}
	}
	iter.Stop()

	log.Printf("Schedule built:\n%v", schedule)

	predictions := bts.MakePredictions(&schedule, *model)
	log.Printf("Made predictions\n%s", predictions)

	// Get picker remaining teams
	iter = fs.Collection("picks").Where("season", "==", seasonDoc.Ref).Where("week", "==", *weekNumber).Limit(1).Documents(ctx)
	picksDoc, err := iter.Next()
	check(err)
	iter.Stop()

	players := make(bts.PlayerMap)
	iter = picksDoc.Ref.Collection("streaks").Documents(ctx)
	for {
		pickDoc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		check(err)

		var ps PickerStreak
		err = pickDoc.DataTo(&ps)
		check(err)

		pickerDoc, err := ps.Picker.Get(ctx)
		check(err)

		var p Picker
		err = pickerDoc.DataTo(&p)
		check(err)

		remainingTeamDocs, err := fs.GetAll(ctx, ps.RemainingTeams)
		check(err)

		remainingTeams := make(bts.Remaining, len(remainingTeamDocs))
		for i, teamDoc := range remainingTeamDocs {
			var team bts.Team
			err = teamDoc.DataTo(&team)
			check(err)

			remainingTeams[i] = team
		}

		players[p.Name], err = bts.NewPlayer(p.Name, remainingTeams, ps.PickTypes)
		check(err)
	}
	iter.Stop()

	log.Printf("Pickers loaded:\n%v", players)

	schedule.FilterWeeks(*weekNumber)
	log.Printf("Filtered schedule:\n%s", schedule)

	predictions.FilterWeeks(*weekNumber)
	log.Printf("Filtered predictions:\n%s", predictions)

	// Here we go.
	// Find the unique users.
	duplicates := players.Duplicates()
	log.Println("The following users are clones of one another:")
	for user, clones := range duplicates {
		log.Printf("%s clones %v", user, clones)
		for _, clone := range clones {
			delete(players, clone)
		}
	}

	// Loop through the unique users
	playerItr := playerIterator(players)

	// Loop through streaks
	ppts := perPlayerTeamStreaks(playerItr, predictions)

	// Update best
	bestStreaks := calculateBestStreaks(ppts)

	// Collect by player
	streakOptions := collectByPlayer(bestStreaks, players, predictions, &schedule)

	// Print results
	for _, streak := range streakOptions {
		j, _ := json.Marshal(streak)
		fmt.Println(string(j))
	}

}

// StreakMap is a simple map of player names to streaks
type streakMap map[playerTeam]streakProb

type streakProb struct {
	streak *bts.Streak
	prob   float64
	spread float64
}

type playerTeam struct {
	player string
	team   bts.Team
}

func (sm *streakMap) update(player string, team bts.Team, spin streakProb) {
	pt := playerTeam{player: player, team: team}
	bestp := math.Inf(-1)
	bests := math.Inf(-1)
	if sp, ok := (*sm)[pt]; ok {
		bestp = sp.prob
		bests = sp.spread
	}
	if spin.prob > bestp || (spin.prob == bestp && spin.spread > bests) {
		(*sm)[pt] = streakProb{streak: spin.streak, prob: spin.prob, spread: spin.spread}
	}
}

func (sm streakMap) getBest(player string) streakProb {
	bestp := math.Inf(-1)
	bests := math.Inf(-1)
	bestt := bts.BYE
	for pt, sp := range sm {
		if pt.player != player {
			continue
		}
		if sp.prob > bestp || (sp.prob == bestp && sp.spread > bests) {
			bestt = pt.team
		}
	}
	return sm[playerTeam{player: player, team: bestt}]
}

type playerTeamStreakProb struct {
	player     *bts.Player
	team       bts.Team
	streakProb streakProb
}

func playerIterator(pm bts.PlayerMap) <-chan *bts.Player {
	out := make(chan *bts.Player)

	go func() {
		defer close(out)

		for _, player := range pm {
			out <- player
		}
	}()

	return out
}

func perPlayerTeamStreaks(ps <-chan *bts.Player, predictions *bts.Predictions) <-chan playerTeamStreakProb {

	out := make(chan playerTeamStreakProb)

	go func() {
		var wg sync.WaitGroup

		for p := range ps {
			wg.Add(1)

			go func(p *bts.Player) {
				for weekOrder := range p.WeekTypeIterator() {
					for teamOrder := range p.RemainingIterator() {
						streak := bts.NewStreak(p.RemainingTeams(), weekOrder, teamOrder)
						prob, spread := bts.SummarizeStreak(predictions, streak)
						if prob <= 0 {
							// Ignore streaks that guarantee a loss.
							continue
						}
						for _, team := range streak.GetWeek(0) {
							sp := streakProb{streak: streak, prob: prob, spread: spread}
							out <- playerTeamStreakProb{player: p, team: team, streakProb: sp}
						}
					}
				}
				wg.Done()
			}(p)

		}
		wg.Wait()
		close(out)
	}()

	return out
}

func calculateBestStreaks(ppts <-chan playerTeamStreakProb) <-chan streakMap {
	out := make(chan streakMap)

	sm := make(streakMap)
	go func() {
		defer close(out)

		for ptsp := range ppts {
			sm.update(ptsp.player.Name(), ptsp.team, ptsp.streakProb)
		}

		out <- sm
	}()

	return out
}

func collectByPlayer(sms <-chan streakMap, players bts.PlayerMap, predictions *bts.Predictions, schedule *bts.Schedule) []PlayerResults {

	// Collect streak options by player
	soByPlayer := make(map[string][]StreakOption)
	for sm := range sms {

		for pt, sp := range sm {

			firstTeam := pt.team
			cProb, cSpread := bts.AccumulateStreak(predictions, sp.streak)
			prob := sp.prob
			spread := sp.spread

			weeks := make([]Week, sp.streak.NumWeeks())
			for iweek := 0; iweek < sp.streak.NumWeeks(); iweek++ {

				seasonWeek := iweek + *weekNumber
				weekProbability := 1.
				weekSpread := 0.

				picks := make([]Pick, 0)
				for _, team := range sp.streak.GetWeek(iweek) {
					probability := predictions.GetProbability(team, iweek)
					spread := predictions.GetSpread(team, iweek)
					opponent := schedule.Get(team, iweek).Team(1)

					picks = append(picks, Pick{Selected: team, Opponent: opponent, Probability: probability, Spread: spread})

					weekProbability *= probability
					weekSpread += spread
				}

				weeks[iweek] = Week{Picks: picks, SeasonWeek: seasonWeek, Probability: weekProbability, Spread: weekSpread}

			}

			so := StreakOption{FirstSelected: firstTeam, Weeks: weeks, CumulativeProbability: cProb, CumulativeSpread: cSpread, Probability: prob, Spread: spread}
			if sos, ok := soByPlayer[pt.player]; ok {
				soByPlayer[pt.player] = append(sos, so)
			} else {
				soByPlayer[pt.player] = []StreakOption{so}
			}

		}

	}

	// Run through players and calculate best option
	prs := make([]PlayerResults, 0)
	for player, streakOptions := range soByPlayer {

		if len(streakOptions) == 0 {
			continue
		}

		sort.Sort(ByProbDesc(streakOptions))

		bestSelection := make([]bts.Team, 0)
		for _, pick := range streakOptions[0].Weeks[0].Picks {
			bestSelection = append(bestSelection, pick.Selected)
		}
		bestProb := streakOptions[0].Probability
		bestSpread := streakOptions[0].Spread

		prs = append(prs, PlayerResults{
			Player:               player,
			StartingWeek:         *weekNumber,
			CalculationStartTime: startTime,
			CalculationEndTime:   time.Now(),
			RemainingTeams:       players[player].RemainingTeams(),
			RemainingWeekTypes:   players[player].RemainingWeekTypes(),
			BestSelection:        bestSelection,
			BestProbability:      bestProb,
			BestSpread:           bestSpread,
			StreakOptions:        streakOptions,
		})
	}

	return prs
}

func determineWeekNumber(players bts.PlayerMap, schedule *bts.Schedule) int {
	guess := -1
	for name, player := range players {
		thisGuess := player.RemainingWeeks()
		if guess >= 0 && thisGuess != guess {
			panic(fmt.Errorf("player %s has an invalid number of weeks remaining: expected %d, found %d", name, thisGuess, guess))
		}
		guess = thisGuess
	}
	week := schedule.NumWeeks() - guess
	return week
}
