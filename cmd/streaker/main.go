package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	bpefs "github.com/reallyasi9/b1gpickem/firestore"
	"google.golang.org/api/iterator"
	yaml "gopkg.in/yaml.v2"
)

var fsclient *firestore.Client

// otherTeams is a mapping of other team names to team DocumentRefs in Firestore.
var otherTeams = make(map[string]*firestore.DocumentRef)

// lukeNames is a mapping of pickers to names Luke has given them.
var lukeNames = make(map[string]*firestore.DocumentRef)

var weekNumber int
var projectID string
var _DRY_RUN bool

func init() {
	flag.IntVar(&weekNumber, "week", -1, "Week of picks starting at 0 for preseason. A value less than 0 will cause the program to attempt to determine the week number based off of Firestore data, and may fail!")
	flag.StringVar(&projectID, "project", "", "Google Cloud Project ID.")
	flag.BoolVar(&_DRY_RUN, "dryrun", false, "Do not write to Firestore, just print what would be written.")
}

// loadTeams loads the team maps, defined above.
func loadTeams(ctx context.Context) {

	teamsItr := fsclient.Collection("teams").Documents(ctx)
	defer teamsItr.Stop()
	for {
		teamDoc, err := teamsItr.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			panic(err) // might not break out of the function--will fail later.
		}

		var team bpefs.Team
		err = teamDoc.DataTo(&team)
		if err != nil {
			panic(err)
		}

		// store by other name
		for _, name := range team.OtherNames {
			if _, exists := otherTeams[name]; exists {
				err = fmt.Errorf("loadTeams: other name \"%s\" is ambiguous: %v", name, team)
				panic(err)
			}
			otherTeams[name] = teamDoc.Ref
			log.Printf("loaded team other name: %s -> %s", name, teamDoc.Ref.ID)
		}
	}
}

// loadPickers loads the picker maps, defined above.
func loadPickers(ctx context.Context) {

	pickersItr := fsclient.Collection("pickers").Documents(ctx)
	defer pickersItr.Stop()
	for {
		pickerDoc, err := pickersItr.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			panic(err) // might not break out of the function--will fail later.
		}

		var picker bpefs.Picker
		err = pickerDoc.DataTo(&picker)
		if err != nil {
			panic(err)
		}

		// store by other name
		name := picker.LukeName
		if _, exists := lukeNames[name]; exists {
			err = fmt.Errorf("loadPickers: luke name \"%s\" is ambiguous: %v", name, picker)
			panic(err)
		}
		lukeNames[name] = pickerDoc.Ref
		log.Printf("loaded picker luke name: %s -> %s", name, pickerDoc.Ref.ID)
	}
}

// mostRecentSeason gets the most recent season from firestore
func mostRecentSeason(ctx context.Context) (*firestore.DocumentRef, error) {
	docItr := fsclient.Collection("seasons").OrderBy("start", firestore.Desc).Limit(1).Documents(ctx)
	defer docItr.Stop()
	seasonDoc, err := docItr.Next()
	if err != nil {
		return nil, err
	}
	log.Printf("most recent season on record: \"%s\"", seasonDoc.Ref.ID)
	return seasonDoc.Ref, nil
}

// mostRecentWeek tries to infer the week number based on all information available
func mostRecentWeek(teamsRemaining RemainingYaml, typesRemaining TypesYaml) (int, error) {
	// Step 1: count everybody's typesRemaining
	guessWeeksRemaining := -1
	pickerTeamsRemaining := make(map[string]int)
	for picker, types := range typesRemaining {
		var remTeams int
		var remWeeks int
		for i, count := range types {
			remTeams += i * count
			remWeeks += count
		}
		if guessWeeksRemaining < 0 {
			guessWeeksRemaining = remWeeks
		} else if guessWeeksRemaining != remWeeks {
			return -1, fmt.Errorf("week types remaining for picker \"%s\" does not match others: %d != %d", picker, remWeeks, guessWeeksRemaining)
		}
		pickerTeamsRemaining[picker] = remTeams
	}
	// Step 2: count everybody's teamsRemaining
	for picker, teams := range teamsRemaining {
		if len(teams) != pickerTeamsRemaining[picker] {
			return -1, fmt.Errorf("count of teams remaining by week type for picker \"%s\" does not match teams remaining in file: %d != %d", picker, pickerTeamsRemaining[picker], len(teams))
		}
	}
	// FIXME: Assumes 14 weeks in a schedule. Should probably be determined by the actual schedule instead!
	if guessWeeksRemaining > 14 {
		return -1, fmt.Errorf("guess of weeks remaining too large: %d > 14", guessWeeksRemaining)
	}
	return 14 - guessWeeksRemaining, nil
}

// RemainingYaml is the the remaining teams YAML file.
// The format of the file is simple: it is an assocaitive list (map) where the top-level keys are luke-names of pickers
// and the values are lists of remaining teams' other-names.
type RemainingYaml map[string][]string

// TypesYaml is the remaining pick types YAML file.
// The format of the file is an associative list (map) where the top-level keys are the luke-names of pickers
// and the values are lists of integers where the index of the list represents the number of picks per week
// (e.g., index zero are bye weeks, index one are single-pick weeks, index two are double-downs, etc.) and
// the values of the list are the number of that type of pick remaining for that picker.
type TypesYaml map[string][]int

func usage() {
	w := flag.CommandLine.Output()
	fmt.Fprint(w, `streaker [flags] <REMAINING.yaml> <WEEKTYPES.yaml>

Update firestore streak picks remaining and week types remaining for pickers.

Arguments:
  <REMAINING.yaml>
	YAML file listing remaining teams for each picker still in Beat The Streak.
  <WEEKTYPES.yaml>
    YAML file listing remaining week types for each picker still in Beat The Streak.

Flags:
`)
	flag.PrintDefaults()
}

func main() {
	ctx := context.Background()

	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 2 {
		usage()
		log.Fatalf("Invalid number of arguments: expected 2, got %d", flag.NArg())
	}

	if projectID == "" {
		// read from environment, else complain to console
		projectID = os.Getenv("GCP_PROJECT")
	}
	if projectID == "" {
		log.Fatalf("Must provide either -project flag or GCP_PROJECT environment variable")
	}

	var err error // avoid shadowing fsclient
	fsclient, err = firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Print(err)
		log.Fatalf("Check that the project ID \"%s\" is correctly specified (either the -project flag or the GCP_PROJECT environment variable)", projectID)
	}

	loadTeams(ctx)
	loadPickers(ctx)
	seasonRef, err := mostRecentSeason(ctx)
	if err != nil {
		log.Fatal(err)
	}

	remainingYaml := flag.Arg(0)
	parsedRemaining := make(map[*firestore.DocumentRef][]*firestore.DocumentRef)
	yr, err := ioutil.ReadFile(remainingYaml)
	if err != nil {
		panic(fmt.Errorf("cannot read remaining YAML file \"%s\": %v", remainingYaml, err))
	}
	var rem RemainingYaml
	err = yaml.Unmarshal(yr, &rem)
	if err != nil {
		panic(fmt.Errorf("cannot unmarshal remaining YAML file \"%s\": %v", remainingYaml, err))
	}
	for userName, remainingTeams := range rem {
		userRef, exists := lukeNames[userName]
		if !exists {
			panic(fmt.Errorf("luke name \"%s\" not in pickers", userName))
		}

		teamsRemaining := make([]*firestore.DocumentRef, len(remainingTeams))
		for i, team := range remainingTeams {
			teamRef, exists := otherTeams[team]
			if !exists {
				panic(fmt.Errorf("team other name \"%s\" not in teams", team))
			}
			teamsRemaining[i] = teamRef
		}
		parsedRemaining[userRef] = teamsRemaining
	}

	typesYaml := flag.Arg(1)
	parsedTypes := make(map[*firestore.DocumentRef][]int)
	yt, err := ioutil.ReadFile(typesYaml)
	if err != nil {
		panic(fmt.Errorf("cannot read types YAML file \"%s\": %v", typesYaml, err))
	}
	var typ TypesYaml
	err = yaml.Unmarshal(yt, &typ)
	if err != nil {
		panic(fmt.Errorf("cannot unmarshal types YAML file \"%s\": %v", typesYaml, err))
	}
	for pickerName, typesRemaining := range typ {
		userRef, exists := lukeNames[pickerName]
		if !exists {
			panic(fmt.Errorf("luke name \"%s\" not in pickers", pickerName))
		}

		parsedTypes[userRef] = typesRemaining
	}

	guessWeeksRemaining, err := mostRecentWeek(rem, typ)
	if weekNumber < 0 {
		if err != nil {
			panic(err)
		}
		log.Printf("Most recent week value calculated to be %d", guessWeeksRemaining)
		weekNumber = guessWeeksRemaining
	}
	if weekNumber != guessWeeksRemaining {
		if err != nil {
			log.Printf("WARN: unable to determine most recent week: %v", err)
		} else {
			log.Printf("WARN: most recent week calculation different than given week number: %d != %d", guessWeeksRemaining, weekNumber)
		}
		log.Printf("Trusting -week flag value %d", weekNumber)
	}

	// Write everything in transaction
	picksRef := fsclient.Collection("streak_teams_remaining").NewDoc()
	picksDoc := bpefs.StreakTeamsRemainingWeek{
		Season: seasonRef,
		Week:   weekNumber,
	}
	streaksColRef := picksRef.Collection("streaks")
	streaksDocs := make([]*firestore.DocumentRef, len(parsedRemaining))
	streaksData := make([]bpefs.StreakTeamsRemaining, len(parsedRemaining))
	i := 0
	for pickerRef, remRefs := range parsedRemaining {
		remDoc := streaksColRef.NewDoc()
		typesRem, ok := parsedTypes[pickerRef]
		if !ok {
			panic(fmt.Errorf("picker \"%s\" does not have types remaining defined", pickerRef.ID))
		}
		streaksDocs[i] = remDoc
		streaksData[i] = bpefs.StreakTeamsRemaining{
			Picker:             pickerRef,
			TeamsRemaining:     remRefs,
			PickTypesRemaining: typesRem,
		}
		i++
	}

	// If dryrun, don't write anything at all!
	if _DRY_RUN {
		log.Print("DRYRUN: would write the following:")
		log.Printf("%s: %+v", picksRef.Path, picksDoc)
		for i := range streaksDocs {
			log.Printf("%s: %+v", streaksDocs[i].Path, streaksData[i])
		}
		log.Print("DONE")
		return
	}

	err = fsclient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		err := tx.Create(picksRef, &picksDoc)
		if err != nil {
			return err
		}
		for i, remDoc := range streaksDocs {
			err := tx.Create(remDoc, &streaksData[i])
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		panic(err)
	}

	log.Printf("DONE")
}
