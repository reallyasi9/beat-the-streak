package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/reallyasi9/beat-the-streak/internal/bts"
	"google.golang.org/api/iterator"
	yaml "gopkg.in/yaml.v2"
)

var fsclient *firestore.Client

var projectID = os.Getenv("GCP_PROJECT")

// otherTeams is a mapping of other team names to team DocumentRefs in Firestore.
var otherTeams = make(map[string]*firestore.DocumentRef)

// byeWeekTeam is a team representing a fake bye week
var byeWeekTeam *firestore.DocumentRef

// Team represents how teams are stored in Firestore
type Team struct {
	OtherNames []string `firestore:"other_names"`
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

// splitLocTeam splits a marked team name into a relative location and a team name.
// Note: this is relative to the schedule team, not the team given here.
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

// YamlSchedule is a representation of a YAML schedule file
type YamlSchedule map[string][]string

// Schedule represents how the data are stored in firestore
type Schedule struct {
	Team      *firestore.DocumentRef   `firestore:"team"`
	Opponents []*firestore.DocumentRef `firestore:"opponents"`
	Locales   []int                    `firestore:"locales"`
}

// SeasonSchedule represents a document in firestore that contains team schedules
type SeasonSchedule struct {
	Season    *firestore.DocumentRef `firestore:"season"`
	Timestamp time.Time              `firestore:"timestamp,serverTimestamp"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("schedule to parse must be passed as an argument")
		os.Exit(1)
	}
	scheduleFile := os.Args[1]
	log.Printf("parsing schedule file \"%s\"", scheduleFile)

	yf, err := ioutil.ReadFile(scheduleFile)
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
	var schedule YamlSchedule
	err = yaml.Unmarshal(yf, &schedule)
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
	log.Printf("read schedule: %v", schedule)

	ctx := context.Background()
	fsclient, err = firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
	loadTeams(ctx)
	byeWeekTeam = fsclient.Collection("teams").Doc("bye week")

	seasonRef, err := mostRecentSeason(ctx)
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}

	schedules := make([]Schedule, 0)
	teamErrors := 0
	for team, opponents := range schedule {
		team1, exists := otherTeams[team]
		if !exists {
			log.Fatalf(`team "%s" not found in teams`, team)
			teamErrors++
			// keep going -- will crash out eventually
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
			team2, exists := otherTeams[other]
			if !exists {
				log.Fatalf(`team "%s" not found in teams`, other)
				teamErrors++
				continue
			}
			opps[i] = team2
			locs[i] = int(loc)
		}

		s := Schedule{
			Team:      team1,
			Opponents: opps,
			Locales:   locs,
		}
		log.Printf("parsed schedule: %v", s)
		schedules = append(schedules, s)
	}
	if teamErrors > 0 {
		log.Fatalf("detected %d team errors", teamErrors)
		os.Exit(2)
	}

	// Write in transaction
	seasonSchedRef := fsclient.Collection("schedules").NewDoc()
	teamSchedCol := seasonSchedRef.Collection("teams")
	seasonSched := SeasonSchedule{
		Season: seasonRef,
	}
	err = fsclient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		err := tx.Create(seasonSchedRef, &seasonSched)
		if err != nil {
			return err
		}

		for _, schedule := range schedules {
			dr := teamSchedCol.NewDoc()
			err := tx.Create(dr, &schedule)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		log.Fatalln(err)
		os.Exit(3)
	}

}
