package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	yaml "gopkg.in/yaml.v2"
)

var fsclient *firestore.Client

var projectID = os.Getenv("GCP_PROJECT")

// otherTeams is a mapping of other team names to team DocumentRefs in Firestore.
var otherTeams = make(map[string]*firestore.DocumentRef)

// lukeNames is a mapping of pickers to names Luke has given them.
var lukeNames = make(map[string]*firestore.DocumentRef)

var remainingYaml = flag.String("remaining", "", "Picker team remaining YAML file.")
var typesYaml = flag.String("types", "", "Picker picks remaining YAML file.")
var weekNumber = flag.Int("week", -1, "Week of picks (starting at 0 for preseason).")

// Team represents how teams are stored in Firestore
type Team struct {
	OtherNames []string `firestore:"other_names"`
}

// Picker represents how pickers are stored in Firestore
type Picker struct {
	NameLuke string `firestore:"name_luke"`
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

		var team Team
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

		var picker Picker
		err = pickerDoc.DataTo(&picker)
		if err != nil {
			panic(err)
		}

		// store by other name
		name := picker.NameLuke
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

// RemainingYaml is the format of the remaining teams YAML file
type RemainingYaml map[string][]string

// TypesYaml is the format of the remaining pick types YAML file
type TypesYaml map[string][]int

// Picks is the document containing user picks remaining in Firestore.
type Picks struct {
	Season    *firestore.DocumentRef `firestore:"season"`
	Week      int                    `firestore:"week"`
	Timestamp time.Time              `firestore:"timestamp,serverTimestamp"`
}

// Streak is how a picker's remaining picks are stored
type Streak struct {
	Picker             *firestore.DocumentRef   `firestore:"picker"`
	Remaining          []*firestore.DocumentRef `firestore:"remaining"`
	PickTypesRemaining []int                    `firestore:"pick_types_remaining"`
}

func main() {
	ctx := context.Background()

	var err error
	fsclient, err = firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalln(err)
		os.Exit(-1)
	}

	flag.Parse()
	if *weekNumber < 0 {
		log.Fatalf("invalid week number %d", *weekNumber)
		os.Exit(1)
	}

	loadTeams(ctx)
	loadPickers(ctx)
	seasonRef, err := mostRecentSeason(ctx)
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}

	parsedRemaining := make(map[*firestore.DocumentRef][]*firestore.DocumentRef)
	yr, err := ioutil.ReadFile(*remainingYaml)
	if err != nil {
		log.Fatalf("error reading remaining YAML file \"%s\": %v", *remainingYaml, err)
	}
	var rem RemainingYaml
	err = yaml.Unmarshal(yr, &rem)
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
	for userName, remainingTeams := range rem {
		userRef, exists := lukeNames[userName]
		if !exists {
			log.Fatalf("luke name \"%s\" not in pickers", userName)
			os.Exit(2)
		}

		teamsRemaining := make([]*firestore.DocumentRef, len(remainingTeams))
		for i, team := range remainingTeams {
			teamRef, exists := otherTeams[team]
			if !exists {
				log.Fatalf("team name \"%s\" not in teams", team)
				os.Exit(3)
			}
			teamsRemaining[i] = teamRef
		}
		parsedRemaining[userRef] = teamsRemaining
	}

	parsedTypes := make(map[*firestore.DocumentRef][]int)
	var typ TypesYaml
	if *typesYaml == "" {
		log.Printf("using default pick types remaining")
	} else {
		yt, err := ioutil.ReadFile(*typesYaml)
		if err != nil {
			log.Printf("cannot read types YAML file \"%s\": ignoring", *typesYaml)
		} else {
			err = yaml.Unmarshal(yt, &typ)
			if err != nil {
				log.Fatalln(err)
				os.Exit(1)
			}
		}
	}
	if typ == nil {
		// fill with default: one pick per week remaining
		for pickerRef, remRef := range parsedRemaining {
			parsedTypes[pickerRef] = []int{0, len(remRef)}
		}
	} else {
		// read from the yaml file
		for pickerName, typesRemaining := range typ {
			userRef, exists := lukeNames[pickerName]
			if !exists {
				log.Fatalf("luke name \"%s\" not in pickers", pickerName)
				os.Exit(2)
			}

			parsedTypes[userRef] = typesRemaining
		}
	}

	// Write everything in transaction
	picksRef := fsclient.Collection("streak_teams_remaining").NewDoc()
	picksDoc := Picks{
		Season: seasonRef,
		Week:   *weekNumber,
	}
	streaksColRef := picksRef.Collection("streaks")
	err = fsclient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		err := tx.Create(picksRef, &picksDoc)
		if err != nil {
			return err
		}

		for pickerRef, remRefs := range parsedRemaining {
			remDoc := streaksColRef.NewDoc()
			typesRem, ok := parsedTypes[pickerRef]
			if !ok {
				return fmt.Errorf("picker \"%s\" does not have types remaining defined", pickerRef.ID)
			}
			s := Streak{
				Picker:             pickerRef,
				Remaining:          remRefs,
				PickTypesRemaining: typesRem,
			}
			err := tx.Create(remDoc, &s)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalln(err)
		os.Exit(4)
	}
}
