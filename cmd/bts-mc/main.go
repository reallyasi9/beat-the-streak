package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"github.com/reallyasi9/beat-the-streak/internal/bts"
	"google.golang.org/api/iterator"
)

func check(w http.ResponseWriter, err error, code int) bool {
	if err != nil {
		log.Fatalln(err)
		http.Error(w, fmt.Sprintf("ERROR %d: %s", code, http.StatusText(code)), code)
		return true
	}
	return false
}

// ModelPerformance holds Firestore data for model performance, parsed from ThePredictionTracker.com
type ModelPerformance struct {
	HomeBias          float64                `firestore:"bias"`
	StandardDeviation float64                `firestore:"std_dev"`
	Model             *firestore.DocumentRef `firestore:"model"`
}

// SagarinScrapeResult stores the home advantages from a scraping of Sagarin.
type SagarinScrapeResult struct {
	HomeAdvantage float64   `firestore:"home_advantage_rating"`
	Timestamp     time.Time `firestore:"timestamp"`
}

// SagarinRating is a rating.  From Sagarin.  Stored in Firestore.  Simple.
type SagarinRating struct {
	Rating float64                `firestore:"rating"`
	Team   *firestore.DocumentRef `firestore:"team"`
}

// TeamSchedule is a team's schedule in Firestore format
type TeamSchedule struct {
	Team              *firestore.DocumentRef   `firestore:"team"`
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

// Week is a week's worth of picks.
type Week struct {
	WeekNumber    int                      `firestore:"week"`
	Pick          []*firestore.DocumentRef `firestore:"pick"`
	Probabilities []float64                `firestore:"probabilities"`
	Spreads       []float64                `firestore:"spreads"`
}

// StreakPrediction is a prediction for a complete streak.
type StreakPrediction struct {
	CumulativeProbability float64 `firestore:"cumulative_probability"`
	CumulativeSpread      float64 `firestore:"cumulative_spread"`
	Weeks                 []Week  `firestore:"weeks"`
}

// ByProbDesc sorts StreakPredictions by probability and spread (descending)
type ByProbDesc []StreakPrediction

func (a ByProbDesc) Len() int { return len(a) }
func (a ByProbDesc) Less(i, j int) bool {
	if a[i].CumulativeProbability == a[j].CumulativeProbability {
		return a[i].CumulativeSpread > a[j].CumulativeSpread
	}
	return a[i].CumulativeProbability > a[j].CumulativeProbability
}
func (a ByProbDesc) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// PickerPrediction contains the collected predictions for a given user.
type PickerPrediction struct {
	Picker            *firestore.DocumentRef `firestore:"picker"`
	Season            *firestore.DocumentRef `firestore:"season"`
	Week              int                    `firestore:"week"`
	Schedule          *firestore.DocumentRef `firestore:"schedule"`
	Sagarin           *firestore.DocumentRef `firestore:"sagarin"`
	PredictionTracker *firestore.DocumentRef `firestore:"prediction_tracker"`

	Remaining []*firestore.DocumentRef `firestore:"remaining"`
	PickTypes []int                    `firestore:"pick_types_remaining"`

	BestPick    []*firestore.DocumentRef `firestore:"best_pick"`
	Probability float64                  `firestore:"probability"`
	Spread      float64                  `firestore:"spread"`

	PossiblePicks []StreakPrediction `firestore:"possible_picks"`

	// CalculationStartTime is when the program that produced the results started
	CalculationStartTime time.Time `firestore:"calculation_start_time"`
	// CalculationEndTime is when the results were generated and finalized
	CalculationEndTime time.Time `firestore:"calculation_end_time"`
}

var teamRefLookup = make(map[string]*firestore.DocumentRef)

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
	}
	return nil
}

func pickerRefLookup(ctx context.Context, fs *firestore.Client, name string) (*firestore.DocumentRef, error) {
	pickerRef, err := fs.Collection("pickers").Where("name_luke", "==", name).Limit(1).Documents(ctx).Next()
	if err != nil {
		return nil, err
	}
	return pickerRef.Ref, nil
}

func main() {
	log.Print("Beating the streak")

	http.HandleFunc("/", handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

// RequestMessage is inside a PubSubMessage
type RequestMessage struct {
	Picker string `json:"picker"`
	Week   *int   `json:"week"` // pointer, as this is optional, but could be zero
}

// PubSubMessage is the payload of a Pub/Sub event.
type PubSubMessage struct {
	Message struct {
		Data []byte `json:"data,omitempty"`
		ID   string `json:"id"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	body, err := ioutil.ReadAll(r.Body)
	if check(w, err, http.StatusBadRequest) {
		return
	}

	var psm PubSubMessage
	err = json.Unmarshal(body, &psm)
	if check(w, err, http.StatusBadRequest) {
		return
	}

	var rm RequestMessage
	err = json.Unmarshal(psm.Message.Data, &rm)
	if check(w, err, http.StatusBadRequest) {
		return
	}

	log.Printf("Beating the streak, picker %s", rm.Picker)
	weekNumber := rm.Week
	pickerName := rm.Picker

	conf := &firebase.Config{}
	app, err := firebase.NewApp(ctx, conf)
	if check(w, err, http.StatusInternalServerError) {
		return
	}

	fs, err := app.Firestore(ctx)
	if check(w, err, http.StatusInternalServerError) {
		return
	}
	defer fs.Close()

	makeTeamLookup(ctx, fs)

	// Get most recent season
	iter := fs.Collection("seasons").OrderBy("start", firestore.Desc).Limit(1).Documents(ctx)
	seasonDoc, err := iter.Next()
	if check(w, err, http.StatusInternalServerError) {
		return
	}
	iter.Stop()

	// Get most recent Sagarin Ratings proper
	iter = fs.Collection("sagarin").OrderBy("timestamp", firestore.Desc).Limit(1).Documents(ctx)
	sagRateDoc, err := iter.Next()
	if check(w, err, http.StatusInternalServerError) {
		return
	}
	iter.Stop()

	// With these in hand, calculate the week number if necessary
	if weekNumber == nil {
		seasonStart, err := seasonDoc.DataAt("start")
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		sagtime, err := sagRateDoc.DataAt("timestamp")
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		weekTime := sagtime.(time.Time).Sub(seasonStart.(time.Time))
		week := int(weekTime.Hours() / (24 * 7))
		weekNumber = &week
		log.Printf("Determined week number %d from Sagarin and season start", *weekNumber)
	} else {
		log.Printf("Week number given as %d", *weekNumber)
	}

	// Get this user
	pickerRef, err := pickerRefLookup(ctx, fs, pickerName)
	if check(w, err, http.StatusInternalServerError) {
		return
	}

	// Get most recent predictions
	iter = fs.Collection("prediction_tracker").OrderBy("timestamp", firestore.Desc).Limit(1).Documents(ctx)
	predictionDoc, err := iter.Next()
	if check(w, err, http.StatusInternalServerError) {
		return
	}
	iter.Stop()

	// Get Sagarin Rating performance
	iter = predictionDoc.Ref.Collection("model_performance").Where("system", "==", "Sagarin Ratings").Limit(1).Documents(ctx)
	sagDoc, err := iter.Next()
	if check(w, err, http.StatusInternalServerError) {
		return
	}
	iter.Stop()

	var sagPerf ModelPerformance
	err = sagDoc.DataTo(&sagPerf)
	if check(w, err, http.StatusInternalServerError) {
		return
	}
	log.Printf("Sagarin Ratings performance: %v", sagPerf)

	homeAdvantage, err := sagRateDoc.DataAt("home_advantage_rating")
	if check(w, err, http.StatusInternalServerError) {
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
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		var sr SagarinRating
		err = teamRatingDoc.DataTo(&sr)
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		log.Printf("Sagarin rating: %v", sr)
		teamRefs = append(teamRefs, sr.Team)
		ratings = append(ratings, sr.Rating)
	}
	iter.Stop()

	teamDocs, err := fs.GetAll(ctx, teamRefs)
	if check(w, err, http.StatusInternalServerError) {
		return
	}

	teams := make([]bts.Team, len(teamDocs))
	for i, td := range teamDocs {
		var team bts.Team
		err := td.DataTo(&team)
		if check(w, err, http.StatusInternalServerError) {
			return
		}

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

	// log.Printf("Most recent season %v", seasonDoc)

	// Get schedule from most recent season
	iter = fs.Collection("schedules").Where("season", "==", seasonDoc.Ref).Limit(1).Documents(ctx)
	scheduleDoc, err := iter.Next()
	if check(w, err, http.StatusInternalServerError) {
		return
	}
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
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		var ts TeamSchedule
		err = teamSchedule.DataTo(&ts)
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		opponentDocs, err := fs.GetAll(ctx, ts.Opponents)
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		teamDoc, err := ts.Team.Get(ctx)
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		var team bts.Team
		err = teamDoc.DataTo(&team)
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		schedule[team] = make([]*bts.Game, len(opponentDocs))
		for i, opponent := range opponentDocs {
			var op bts.Team
			err = opponent.DataTo(&op)
			if check(w, err, http.StatusInternalServerError) {
				return
			}

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
	iter = fs.Collection("picks").Where("season", "==", seasonDoc.Ref).Where("week", "==", weekNumber).Limit(1).Documents(ctx)
	picksDoc, err := iter.Next()
	if check(w, err, http.StatusInternalServerError) {
		return
	}
	iter.Stop()

	players := make(bts.PlayerMap)
	// for fast lookups later
	iter = picksDoc.Ref.Collection("streaks").Where("picker", "==", pickerRef).Limit(1).Documents(ctx)
	pickDoc, err := iter.Next()
	if check(w, err, http.StatusInternalServerError) {
		return
	}
	iter.Stop()

	var ps PickerStreak
	err = pickDoc.DataTo(&ps)
	if check(w, err, http.StatusInternalServerError) {
		return
	}

	pickerDoc, err := ps.Picker.Get(ctx)
	if check(w, err, http.StatusInternalServerError) {
		return
	}

	var p Picker
	err = pickerDoc.DataTo(&p)
	if check(w, err, http.StatusInternalServerError) {
		return
	}

	remainingTeamDocs, err := fs.GetAll(ctx, ps.RemainingTeams)
	if check(w, err, http.StatusInternalServerError) {
		return
	}

	remainingTeams := make(bts.Remaining, len(remainingTeamDocs))
	for i, teamDoc := range remainingTeamDocs {
		var team bts.Team
		err = teamDoc.DataTo(&team)
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		remainingTeams[i] = team
	}

	players[p.Name], err = bts.NewPlayer(p.Name, remainingTeams, ps.PickTypes)
	if check(w, err, http.StatusInternalServerError) {
		return
	}

	log.Printf("Pickers loaded:\n%v", players)

	schedule.FilterWeeks(*weekNumber)
	log.Printf("Filtered schedule:\n%s", schedule)

	predictions.FilterWeeks(*weekNumber)
	log.Printf("Filtered predictions:\n%s", predictions)

	// Here we go.
	// Find the unique users.
	// Legacy code!
	duplicates := players.Duplicates()
	log.Println("The following users are clones of one another:")
	for user, clones := range duplicates {
		log.Printf("%s clones %v", user, clones)
		for _, clone := range clones {
			delete(players, clone)
		}
	}

	log.Println("Starting MC")

	// Loop through the unique users
	playerItr := playerIterator(players)

	// Loop through streaks
	ppts := perPlayerTeamStreaks(playerItr, predictions)

	// Update best
	bestStreaks := calculateBestStreaks(ppts)

	// Collect by player
	streakOptions := collectByPlayer(bestStreaks, players, predictions, &schedule, *weekNumber)

	// Print results
	output := fs.Collection("streak_predictions")
	// Note: only one picker for now!
	for _, streak := range streakOptions {
		streak.Picker = ps.Picker
		streak.Remaining = ps.RemainingTeams
		streak.PickTypes = ps.PickTypes
		streak.Schedule = scheduleDoc.Ref
		streak.Sagarin = sagRateDoc.Ref
		streak.Season = seasonDoc.Ref
		streak.PredictionTracker = predictionDoc.Ref

		log.Printf("Writing:\n%v", streak)

		_, wr, err := output.Add(ctx, streak)
		if check(w, err, http.StatusInternalServerError) {
			return
		}
		log.Printf("Wrote streak %v", wr)
	}

	http.Error(w, http.StatusText(http.StatusOK), http.StatusOK)

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

func collectByPlayer(sms <-chan streakMap, players bts.PlayerMap, predictions *bts.Predictions, schedule *bts.Schedule, weekNumber int) map[string]PickerPrediction {

	startTime := time.Now()

	// Collect streak options by player
	soByPlayer := make(map[string][]StreakPrediction)
	for sm := range sms {

		for pt, sp := range sm {

			prob := sp.prob
			spread := sp.spread

			weeks := make([]Week, sp.streak.NumWeeks())
			for iweek := 0; iweek < sp.streak.NumWeeks(); iweek++ {

				seasonWeek := iweek + weekNumber
				pickedTeams := make([]*firestore.DocumentRef, 0)
				pickedProbs := make([]float64, 0)
				pickedSpreads := make([]float64, 0)
				for _, team := range sp.streak.GetWeek(iweek) {
					probability := predictions.GetProbability(team, iweek)
					pickedProbs = append(pickedProbs, probability)

					spread := predictions.GetSpread(team, iweek)
					pickedSpreads = append(pickedSpreads, spread)

					// opponent := schedule.Get(team, iweek).Team(1)
					pickedTeams = append(pickedTeams, teamRefLookup[team.Name()])
				}

				weeks[iweek] = Week{WeekNumber: seasonWeek, Pick: pickedTeams, Probabilities: pickedProbs, Spreads: pickedSpreads}

			}

			so := StreakPrediction{CumulativeProbability: prob, CumulativeSpread: spread, Weeks: weeks}
			if sos, ok := soByPlayer[pt.player]; ok {
				soByPlayer[pt.player] = append(sos, so)
			} else {
				soByPlayer[pt.player] = []StreakPrediction{so}
			}

		}

	}

	// Run through players and calculate best option
	prs := make(map[string]PickerPrediction)
	for picker, streakOptions := range soByPlayer {
		// TODO: look up player (key of soByPlayer)

		if len(streakOptions) == 0 {
			continue
		}

		sort.Sort(ByProbDesc(streakOptions))

		bestSelection := streakOptions[0].Weeks[0].Pick
		bestProb := streakOptions[0].CumulativeProbability
		bestSpread := streakOptions[0].CumulativeSpread

		prs[picker] = PickerPrediction{
			// Picker            *firestore.DocumentRef `firestore:"picker"`
			// Season            *firestore.DocumentRef `firestore:"season"`
			Week: weekNumber,
			// Schedule          *firestore.DocumentRef `firestore:"schedule"`
			// Sagarin           *firestore.DocumentRef `firestore:"sagarin"`
			// PredictionTracker *firestore.DocumentRef `firestore:"prediction_tracker"`

			// Remaining []*firestore.DocumentRef `firestore:"remaining"`
			// PickTypes []int                    `firestore:"pick_types_remaining"`

			BestPick:             bestSelection,
			Probability:          bestProb,
			Spread:               bestSpread,
			PossiblePicks:        streakOptions,
			CalculationStartTime: startTime,
			CalculationEndTime:   time.Now(),
		}
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
