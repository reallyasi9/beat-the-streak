package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"github.com/reallyasi9/beat-the-streak/internal/bts"
	"google.golang.org/api/iterator"

	bpefs "github.com/reallyasi9/b1gpickem/firestore"
)

func check(w http.ResponseWriter, err error, code int) bool {
	if err != nil {
		log.Fatalln(err)
		http.Error(w, fmt.Sprintf("ERROR %d: %s", code, http.StatusText(code)), code)
		return true
	}
	return false
}

// Picker is a picker.  Huh.
type Picker struct {
	Name string `firestore:"name_luke"`
}

// ByProbDesc sorts StreakPredictions by probability and spread (descending)
type ByProbDesc []bpefs.StreakPrediction

func (a ByProbDesc) Len() int { return len(a) }
func (a ByProbDesc) Less(i, j int) bool {
	if a[i].CumulativeProbability == a[j].CumulativeProbability {
		return a[i].CumulativeSpread > a[j].CumulativeSpread
	}
	return a[i].CumulativeProbability > a[j].CumulativeProbability
}
func (a ByProbDesc) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

var teamRefLookup = make(map[bts.Team]*firestore.DocumentRef)

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

		teamRefLookup[bts.Team(team4.(string))] = teamDoc.Ref
	}
	return nil
}

func pickerRefsLookup(ctx context.Context, fs *firestore.Client, names []string, all bool) ([]*firestore.DocumentRef, error) {
	pickers := make([]*firestore.DocumentRef, 0, len(names))
	if all {
		snapshots, err := fs.Collection("pickers").Documents(ctx).GetAll()
		if err != nil {
			return nil, err
		}
		for _, s := range snapshots {
			pickers = append(pickers, s.Ref)
		}
		return pickers, nil
	}

	for _, name := range names {
		pickerRef, err := fs.Collection("pickers").Where("name_luke", "==", name).Limit(1).Documents(ctx).Next()
		if err != nil {
			return nil, err
		}
		pickers = append(pickers, pickerRef.Ref)
	}
	return pickers, nil
}

var weekFlag = flag.Int("week", -1, "Week number. Negative values will calculate week number based on today's date.")
var maxItr = flag.Int("maxi", 100000000, "Number of simulated annealing iterations per worker.")
var tC = flag.Float64("tc", 1., "Simulated annealing temperature constant: p = (tc * (maxi - i) / maxi)^te.")
var tE = flag.Float64("te", 3., "Simulated annealing temperature exponent: p = (tc * (maxi - i) / maxi)^te.")
var resetItr = flag.Int("reseti", 10000, "Maximum number of iterations to allow simulated annealing solution to wonder before resetting to best solution found so far.")
var seed = flag.Int64("seed", -1, "Seed for RNG governing simulated annealing process. Negative values will use system clock to seed RNG.")
var workers = flag.Int("workers", 1, "Number of workers per simulated picker. Increases odds of finding the global maximum.")
var doAll = flag.Bool("all", false, "Ignore picker list and simulate all registered pickers still in the streak.")
var _DRY_RUN = flag.Bool("dryrun", false, "Rather than write the output to Firestore, just report what would have been written.")

func usage() {
	w := flag.CommandLine.Output()
	fmt.Fprint(w, `bts-mc [flags] [Picker [Picker ...]]

Parse a B1G Pick 'Em slate.

Arguments:
  [[Picker] [Picker ...]]
	Luke-given name of picker to simulate. Can list multiple pickers. Omitting this argument will start a PubSub listener on port ENV["PORT"].

Flags:
`)
	flag.PrintDefaults()
}

func mockRequest(pickers []string, week *int) (*httptest.ResponseRecorder, *http.Request) {
	rm := RequestMessage{Pickers: pickers, Week: week}
	rmJson, err := json.Marshal(rm)
	if err != nil {
		log.Fatal(err)
	}

	psm := PubSubMessage{
		Message: innerMessage{
			Data: rmJson,
			ID:   "example-id",
		},
		Subscription: "example-subscription",
	}
	psmJson, _ := json.Marshal(psm)
	reqBody := bytes.NewReader((psmJson))

	req := httptest.NewRequest("POST", "https://example.com/foo", reqBody)
	w := httptest.NewRecorder()

	return w, req
}

func main() {
	flag.Usage = usage
	flag.Parse()

	log.Print("Beating the streak")

	pickers := flag.Args()

	if len(pickers) != 0 || *doAll {
		log.Printf("Mocking HTTP request for pickers %v week %d", pickers, *weekFlag)
		w, req := mockRequest(pickers, weekFlag)
		handler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != 200 {
			err := fmt.Errorf("status code %d: %s", resp.StatusCode, body)
			log.Fatal(err)
		}

		return
	}

	http.HandleFunc("/", handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

// RequestMessage is inside a PubSubMessage
type RequestMessage struct {
	// Pickers are the pickers to simulate. An empty slice means simulate all the active pickers.
	Pickers []string `json:"pickers"`

	Week *int `json:"week"` // pointer, as this is optional, but could be zero
}

// InnerMessage is the inner payload of a Pub/Sub event.
type innerMessage struct {
	Data []byte `json:"data,omitempty"`
	ID   string `json:"id"`
}

// PubSubMessage is the payload of a Pub/Sub event.
type PubSubMessage struct {
	Message      innerMessage `json:"message"`
	Subscription string       `json:"subscription"`
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

	log.Printf("Beating the streak, pickers %s", rm.Pickers)
	weekNumber := rm.Week
	pickerNames := rm.Pickers

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
	log.Printf("latest season discovered: %s", seasonDoc.Ref.ID)

	// Get most recent Sagarin Ratings proper
	iter = fs.Collection("sagarin").OrderBy("timestamp", firestore.Desc).Limit(1).Documents(ctx)
	sagRateDoc, err := iter.Next()
	if check(w, err, http.StatusInternalServerError) {
		return
	}
	iter.Stop()
	log.Printf("latest sagarin ratings discovered: %s", sagRateDoc.Ref.ID)

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
	pickerRefs, err := pickerRefsLookup(ctx, fs, pickerNames, *doAll)
	if check(w, err, http.StatusInternalServerError) {
		return
	}
	log.Printf("pickers loaded: %+v", pickerRefs)

	// Get most recent predictions
	iter = fs.Collection("prediction_tracker").OrderBy("timestamp", firestore.Desc).Limit(1).Documents(ctx)
	predictionDoc, err := iter.Next()
	if check(w, err, http.StatusInternalServerError) {
		return
	}
	iter.Stop()
	log.Printf("latest prediction tracker discovered: %s", predictionDoc.Ref.ID)

	// Get Sagarin Rating performance
	iter = predictionDoc.Ref.Collection("model_performance").Where("system", "==", "Sagarin Points").Limit(1).Documents(ctx)
	sagDoc, err := iter.Next()
	if check(w, err, http.StatusInternalServerError) {
		return
	}
	iter.Stop()
	log.Printf("sagarin model performances discovered: %s", sagDoc.Ref.ID)

	var sagPerf bpefs.ModelPerformance
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

		var sr bpefs.SagarinRating
		err = teamRatingDoc.DataTo(&sr)
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		//log.Printf("Sagarin rating: %v", sr)
		teamRefs = append(teamRefs, sr.Team)
		ratings = append(ratings, sr.Rating)
	}
	iter.Stop()
	log.Printf("team ratings filled")

	teamDocs, err := fs.GetAll(ctx, teamRefs)
	if check(w, err, http.StatusInternalServerError) {
		return
	}

	teams := make([]bts.Team, len(teamDocs))
	for i, td := range teamDocs {
		var team bpefs.Team
		err := td.DataTo(&team)
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		// log.Printf("team %v", team)
		teams[i] = bts.Team(team.Name4)
	}

	// Build the probability model
	ratingsMap := make(map[bts.Team]float64)
	for i, t := range teams {
		ratingsMap[t] = ratings[i]
	}
	homeBias := sagPerf.Bias + homeAdvantage.(float64)
	closeBias := homeBias / 2.
	model := bts.NewGaussianSpreadModel(ratingsMap, sagPerf.StdDev, homeBias, closeBias)

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

		var ts bpefs.Schedule
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

		var team bpefs.Team
		err = teamDoc.DataTo(&team)
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		t := bts.Team(team.Name4)
		schedule[t] = make([]*bts.Game, len(opponentDocs))
		for i, opponent := range opponentDocs {
			var op bpefs.Team
			err = opponent.DataTo(&op)
			if check(w, err, http.StatusInternalServerError) {
				return
			}

			o := bts.Team(op.Name4)
			game := bts.NewGame(t, o, bts.RelativeLocation(ts.RelativeLocations[i]))
			//log.Printf("game loaded %v", game)
			schedule[t][i] = game

		}
	}
	iter.Stop()

	log.Printf("Schedule built:\n%v", schedule)

	predictions := bts.MakePredictions(&schedule, *model)
	log.Printf("Made predictions\n%s", predictions)

	// Get picker remaining teams
	log.Printf("Loading picks for season %s, week %d", seasonDoc.Ref.ID, *weekNumber)
	iter = fs.Collection("streak_teams_remaining").Where("season", "==", seasonDoc.Ref).Where("week", "==", *weekNumber).OrderBy("timestamp", firestore.Desc).Limit(1).Documents(ctx)
	picksDoc, err := iter.Next()
	if check(w, err, http.StatusInternalServerError) {
		return
	}
	iter.Stop()
	log.Printf("Picks loaded: %s", picksDoc.Ref.ID)

	// for fast lookups later
	players := make(bts.PlayerMap)

	for _, pickerRef := range pickerRefs {
		iter = picksDoc.Ref.Collection("streaks").Where("picker", "==", pickerRef).Limit(1).Documents(ctx)
		pickDoc, err := iter.Next()
		if err == iterator.Done {
			log.Printf("Picker %s has no streaks", pickerRef.ID)
			continue
		}
		if check(w, err, http.StatusInternalServerError) {
			return
		}
		iter.Stop()
		log.Printf("Streak loaded: %s", pickDoc.Ref.ID)

		var ps bpefs.StreakPredictions
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

		remainingTeamDocs, err := fs.GetAll(ctx, ps.TeamsRemaining)
		if check(w, err, http.StatusInternalServerError) {
			return
		}

		remainingTeams := make(bts.Remaining, len(remainingTeamDocs))
		for i, teamDoc := range remainingTeamDocs {
			var team bpefs.Team
			err = teamDoc.DataTo(&team)
			if check(w, err, http.StatusInternalServerError) {
				return
			}

			remainingTeams[i] = bts.Team(team.Name4)
		}

		players[p.Name], err = bts.NewPlayer(p.Name, ps.Picker, remainingTeams, ps.TeamsRemaining, ps.PickTypesRemaining)
		if check(w, err, http.StatusInternalServerError) {
			return
		}
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
	log.Println("The following pickers are unique clones of one another:")
	for user, clones := range duplicates {
		if len(clones) == 0 {
			log.Printf("%s is unique", user)
		} else {
			log.Printf("%s clones %v", user, clones)
		}
		for _, clone := range clones {
			delete(players, clone.Name())
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
	streakOptions := collectByPlayer(bestStreaks, players, predictions, &schedule, *weekNumber, duplicates)

	// Print results
	output := fs.Collection("streak_predictions")

	if *_DRY_RUN {
		log.Print("DRY RUN: Would write the following:")
	}
	for _, streak := range streakOptions {
		streak.Schedule = scheduleDoc.Ref
		streak.Sagarin = sagRateDoc.Ref
		streak.Season = seasonDoc.Ref
		streak.PredictionTracker = predictionDoc.Ref

		if *_DRY_RUN {
			log.Printf("%s: add %+v", output.Path, streak)
			continue
		}

		log.Printf("Writing:\n%+v", streak)

		_, _, err := output.Add(ctx, streak)
		if check(w, err, http.StatusInternalServerError) {
			return
		}
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

	out := make(chan playerTeamStreakProb, 100)

	go func(out chan<- playerTeamStreakProb) {
		var wg sync.WaitGroup
		sd := *seed
		if sd < 0 {
			sd = time.Now().UnixNano()
		}
		src := rand.NewSource(sd)
		for p := range ps {
			for i := 0; i < *workers; i++ {
				wg.Add(1)
				mySeed := src.Int63()
				go func(worker int, p *bts.Player, out chan<- playerTeamStreakProb) {
					anneal(mySeed, worker, p, predictions, out)
					wg.Done()
				}(i, p, out)
			}
		}
		wg.Wait()
		close(out)
	}(out)

	return out
}

func anneal(seed int64, worker int, p *bts.Player, predictions *bts.Predictions, out chan<- playerTeamStreakProb) {

	src := rand.NewSource(seed)
	rng := rand.New(src)

	maxIterations := *maxItr
	tConst := *tC
	tExp := *tE
	maxDrift := *resetItr
	countSinceReset := maxDrift

	s := bts.NewStreak(p.RemainingTeams(), <-p.WeekTypeIterator())
	bestS := s.Clone()
	resetS := s.Clone()
	bestP := 0.
	resetP := 0.
	bestSpread := 0.
	resetSpread := 0.

	log.Printf("Player %s w %d start: p=%f, s=%f, streak=%s", p.Name(), worker, bestP, bestSpread, bestS)
	for i := 0; i < maxIterations; i++ {
		temperature := tConst * float64(maxIterations-i) / float64(maxIterations)
		temperature = math.Pow(temperature, tExp)

		s.Perturbate(src, true)
		newP, newSpread := bts.SummarizeStreak(predictions, s)

		// ignore impossible outcomes
		if newP == 0 {
			continue
		}

		if newP > bestP || (newP == bestP && newSpread > bestSpread) || (bestP-newP)*temperature > rng.Float64() {

			// if newP <= bestP {
			// 	log.Printf("Player %s accepted worse outcome due to temperature", p.Name())
			// }

			bestP = newP
			bestSpread = newSpread
			bestS = s.Clone()

			if bestP > resetP {
				resetP = bestP
				resetSpread = bestSpread
				resetS = bestS.Clone()
				countSinceReset = maxDrift

				for _, team := range resetS.GetWeek(0) {
					sp := streakProb{streak: resetS.Clone(), prob: resetP, spread: resetSpread}
					out <- playerTeamStreakProb{player: p, team: team, streakProb: sp}
				}

				log.Printf("Player %s w %d itr %d (temp %f): p=%f, s=%f, streak=%s", p.Name(), worker, i, temperature, bestP, bestSpread, bestS)
			}

		} else if countSinceReset < 0 {
			countSinceReset = maxDrift
			bestP = resetP
			bestSpread = resetSpread
			// bestS = resetS.Clone()
			s = resetS.Clone()

			// log.Printf("Player %s reset at itr %d to p=%f, s=%f, streak=%s", p.Name(), i, bestP, bestSpread, bestS)
		}

		countSinceReset--
	}
}

func calculateBestStreaks(ppts <-chan playerTeamStreakProb) <-chan streakMap {
	out := make(chan streakMap, 100)

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

func collectByPlayer(sms <-chan streakMap, players bts.PlayerMap, predictions *bts.Predictions, schedule *bts.Schedule, weekNumber int, duplicates map[string][]*bts.Player) map[string]bpefs.StreakPredictions {

	startTime := time.Now()

	// Collect streak options by player
	soByPlayer := make(map[string][]bpefs.StreakPrediction)
	for sm := range sms {

		for pt, sp := range sm {

			prob := sp.prob
			spread := sp.spread

			weeks := make([]bpefs.StreakWeek, sp.streak.NumWeeks())
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
					pickedTeams = append(pickedTeams, teamRefLookup[team])
				}

				weeks[iweek] = bpefs.StreakWeek{Week: seasonWeek, Pick: pickedTeams, Probabilities: pickedProbs, Spreads: pickedSpreads}

			}

			so := bpefs.StreakPrediction{CumulativeProbability: prob, CumulativeSpread: spread, Weeks: weeks}
			soByPlayer[pt.player] = append(soByPlayer[pt.player], so)

			// duplicate results
			for _, dupPlayer := range duplicates[pt.player] {
				soByPlayer[dupPlayer.Name()] = append(soByPlayer[dupPlayer.Name()], so)
				// now that the simulation is done, add the duplicates back to the player map
				players[dupPlayer.Name()] = dupPlayer
			}
		}

	}

	// Run through players and calculate best option
	prs := make(map[string]bpefs.StreakPredictions)
	for picker, streakOptions := range soByPlayer {
		// TODO: look up player (key of soByPlayer)

		if len(streakOptions) == 0 {
			continue
		}

		sort.Sort(ByProbDesc(streakOptions))

		bestSelection := streakOptions[0].Weeks[0].Pick
		bestProb := streakOptions[0].CumulativeProbability
		bestSpread := streakOptions[0].CumulativeSpread

		prs[picker] = bpefs.StreakPredictions{
			Picker: players[picker].Ref(),
			// Picker            *firestore.DocumentRef `firestore:"picker"`
			// Season            *firestore.DocumentRef `firestore:"season"`
			Week: weekNumber,
			// Schedule          *firestore.DocumentRef `firestore:"schedule"`
			// Sagarin           *firestore.DocumentRef `firestore:"sagarin"`
			// PredictionTracker *firestore.DocumentRef `firestore:"prediction_tracker"`

			TeamsRemaining:     players[picker].RemainingTeamsRefs(),
			PickTypesRemaining: players[picker].RemainingWeekTypes(),

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

// func determineWeekNumber(players bts.PlayerMap, schedule *bts.Schedule) int {
// 	guess := -1
// 	for name, player := range players {
// 		thisGuess := player.RemainingWeeks()
// 		if guess >= 0 && thisGuess != guess {
// 			panic(fmt.Errorf("player %s has an invalid number of weeks remaining: expected %d, found %d", name, thisGuess, guess))
// 		}
// 		guess = thisGuess
// 	}
// 	week := schedule.NumWeeks() - guess
// 	return week
// }
