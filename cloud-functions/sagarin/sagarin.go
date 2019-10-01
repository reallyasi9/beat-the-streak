package sagarin

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"cloud.google.com/go/errorreporting"
	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2beta3"
)

// homeAdvRE parses Sagarin output for the home advantage line.
var homeAdvRE = regexp.MustCompile("HOME ADVANTAGE=" +
	"\\[<font color=\"#9900ff\">\\s*([\\-0-9.]+)</font>\\]\\s*" + // RATING
	"\\[<font color=\"#0000ff\">\\s*([\\-0-9.]+)</font>\\]\\s*" + // POINTS
	"\\[<font color=\"#ff0000\">\\s*([\\-0-9.]+)</font>\\]\\s*" + // GOLDEN_MEAN
	"\\[<font color=\"#4cc417\">\\s*([\\-0-9.]+)</font>\\]") // RECENT

// ratingsRE parses Sagarin output for each team's rating.
var ratingsRE = regexp.MustCompile("<font color=\"#000000\">\\s*" +
	"\\d+\\s+" + // rank
	"(.*?)\\s+" + // name
	"[A]+\\s*=</font>" + // league
	"<font color=\"#9900ff\">\\s*([\\-0-9.]+)</font>.*?" + // RATING
	"<font color=\"#0000ff\">\\s*([\\-0-9.]+)\\s.*?</font>.*?" + // POINTS
	"<font color=\"#ff0000\">\\s*([\\-0-9.]+)\\s.*?</font>.*?" + // GOLDEN_MEAN
	"<font color=\"#4cc417\">\\s*([\\-0-9.]+)\\s.*?</font>") // RECENT

// once makes sure functions are only run once.
var once sync.Once

var fsclient *firestore.Client

var erclient *errorreporting.Client

var ctclient *cloudtasks.Client

var projectID = os.Getenv("GCP_PROJECT")

// otherTeams is a mapping of other team names to team DocumentRefs in Firestore.
var otherTeams = make(map[string]*firestore.DocumentRef)

// checkErr checks and reports on an error, returning true if an error occurred.
func checkErr(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if erclient == nil {
		var err2 error
		erclient, err2 = errorreporting.NewClient(ctx, projectID, errorreporting.Config{
			ServiceName: os.Getenv("FUNCTION_NAME"),
			OnError: func(err error) {
				log.Printf("Could not log error: %v", err)
			},
		})
		if err2 != nil {
			panic(err2)
		}
	}
	erclient.Report(errorreporting.Entry{
		Error: err,
	})
	log.Print(err)
	return true
}

// Team represents how teams are stored in Firestore
type Team struct {
	OtherNames []string `firestore:"other_names"`
}

// loadTeams loads the team maps, defined above.  Must be niladic to be called by Once.Do, so panics on failure.
func loadTeams(ctx context.Context) {

	teamsItr := fsclient.Collection("teams").Documents(ctx)
	defer teamsItr.Stop()
	for {
		teamDoc, err := teamsItr.Next()
		if err == iterator.Done {
			break
		}
		if checkErr(ctx, err) {
			panic(err) // might not break out of the function--will fail later.
		}

		var team Team
		err = teamDoc.DataTo(&team)
		if checkErr(ctx, err) {
			panic(err)
		}

		// store by other name
		for _, name := range team.OtherNames {
			if _, exists := otherTeams[name]; exists {
				err = fmt.Errorf("loadTeams: other name \"%s\" is ambiguous: %v", name, team)
				checkErr(ctx, err)
				panic(err)
			}
			otherTeams[name] = teamDoc.Ref
			log.Printf("loaded team other name: %s -> %s", name, teamDoc.Ref.ID)
		}
	}
}

// PubSubMessage is the payload of a Pub/Sub event.
type PubSubMessage struct {
	Data []byte `json:"data"`
}

// ScrapeMessage is a representation of the expected message from Pub/Sub
type ScrapeMessage struct {
	SagarinURL string `json:"url"`
}

// HomeAdvantage represents Sagarin home advantages and team scores for one scraping of the website.
type HomeAdvantage struct {
	Timestamp  time.Time `firestore:"timestamp,serverTimestamp"`
	Rating     float64   `firestore:"home_advantage_rating"`
	Points     float64   `firestore:"home_advantage_points"`
	GoldenMean float64   `firestore:"home_advantage_golden_mean"`
	Recent     float64   `firestore:"home_advantage_recent"`
}

// ScrapeSagarin consumes a Pub/Sub message
func ScrapeSagarin(ctx context.Context, m PubSubMessage) error {

	var err error
	if fsclient == nil {
		fsclient, err = firestore.NewClient(ctx, projectID)
		if checkErr(ctx, err) {
			return err
		}
	}

	// Load the teams and the models, but only once!
	once.Do(func() { loadTeams(ctx) })

	var message ScrapeMessage
	err = json.Unmarshal(m.Data, &message)
	if checkErr(ctx, err) {
		return err
	}

	log.Printf("received path from pub/sub \"%s\"", message)

	log.Printf("parsing sagarin ratings from URL \"%s\"", message.SagarinURL)
	advs, ratings, err := parseSagarinTable(message.SagarinURL)
	if checkErr(ctx, err) {
		return err
	}

	// Write everything in transaction, but keep writes under wrap to avoid 500 field update limit...
	sagColRef := fsclient.Collection("sagarin")
	sagDocRef := sagColRef.NewDoc()
	ratingsColRef := sagDocRef.Collection("ratings")
	_, err = sagDocRef.Create(ctx, advs)
	if checkErr(ctx, err) {
		return err
	}
	// first 100
	err = fsclient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		for _, rating := range ratings[:100] {
			docRef := ratingsColRef.NewDoc()
			err = tx.Create(docRef, &rating)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if checkErr(ctx, err) {
		return err
	}

	// all the rest
	err = fsclient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		for _, rating := range ratings[100:] {
			docRef := ratingsColRef.NewDoc()
			err = tx.Create(docRef, &rating)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if checkErr(ctx, err) {
		return err
	}

	log.Printf("done")

	return nil
}

// Rating represents a team's Sagarin ratings in firestore.
type Rating struct {
	Team       *firestore.DocumentRef `firestore:"team"`
	Rating     float64                `firestore:"rating"`
	Points     float64                `firestore:"points"`
	GoldenMean float64                `firestore:"golden_mean"`
	Recent     float64                `firestore:"recent"`
}

// linkTeam links the team by its "other" name, stored in r, to a proper document reference.
func linkTeam(n string) (*firestore.DocumentRef, error) {
	ref, exists := otherTeams[n]
	if !exists {
		return nil, fmt.Errorf("linkTeam: team name \"%s\" not found in teams", n)
	}
	log.Printf("linking team \"%s\" -> %s", n, ref.ID)
	return ref, nil
}

// parseSagarinTable parses the table provided by Sagarin for each team.
func parseSagarinTable(url string) (*HomeAdvantage, []Rating, error) {
	// <sigh> Oh Sagarin...
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Get(url)
	if err != nil {
		return nil, nil, fmt.Errorf("parseSagarinTable: cannot get URL \"%s\": %v", url, err)
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("parseSagarinTable: cannot read body from URL \"%s\": %v", url, err)
	}

	bodyString := string(content)

	homeMatches := homeAdvRE.FindStringSubmatch(bodyString)
	if len(homeMatches) != 5 { // 4 advantages + full match
		return nil, nil, fmt.Errorf("parseSagarinTable: cannot find home advantage line in \"%s\"", url)
	}

	ratingAdv, err := strconv.ParseFloat(homeMatches[1], 64)
	if err != nil {
		return nil, nil, fmt.Errorf("parseSagarinTable: cannot parse string \"%s\" as float: %v", homeMatches[1], err)
	}

	pointsAdv, err := strconv.ParseFloat(homeMatches[2], 64)
	if err != nil {
		return nil, nil, fmt.Errorf("parseSagarinTable: cannot parse string \"%s\" as float: %v", homeMatches[2], err)
	}

	goldenMeanAdv, err := strconv.ParseFloat(homeMatches[3], 64)
	if err != nil {
		return nil, nil, fmt.Errorf("parseSagarinTable: cannot parse string \"%s\" as float: %v", homeMatches[3], err)
	}

	recentAdv, err := strconv.ParseFloat(homeMatches[4], 64)
	if err != nil {
		return nil, nil, fmt.Errorf("parseSagarinTable: cannot parse string \"%s\" as float: %v", homeMatches[4], err)
	}

	adv := HomeAdvantage{
		Rating:     ratingAdv,
		Points:     pointsAdv,
		GoldenMean: goldenMeanAdv,
		Recent:     recentAdv,
	}
	log.Printf("parsed home advantages: %v", adv)

	teamMatches := ratingsRE.FindAllStringSubmatch(bodyString, -1)
	if len(teamMatches) == 0 {
		return nil, nil, fmt.Errorf("parseSagarinTable: cannot find team lines in \"%s\"", url)
	}
	ratings := make([]Rating, len(teamMatches))
	for i, match := range teamMatches {
		name := match[1]
		teamRef, err := linkTeam(name)
		if err != nil {
			return nil, nil, fmt.Errorf("parseSagarinTable: %v", err)
		}

		rating, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			return nil, nil, fmt.Errorf("parseSagarinTable: cannot parse string \"%s\" as float: %v", match[2], err)
		}

		points, err := strconv.ParseFloat(match[3], 64)
		if err != nil {
			return nil, nil, fmt.Errorf("parseSagarinTable: cannot parse string \"%s\" as float: %v", match[3], err)
		}

		goldenMean, err := strconv.ParseFloat(match[4], 64)
		if err != nil {
			return nil, nil, fmt.Errorf("parseSagarinTable: cannot parse string \"%s\" as float: %v", match[4], err)
		}

		recent, err := strconv.ParseFloat(match[5], 64)
		if err != nil {
			return nil, nil, fmt.Errorf("parseSagarinTable: cannot parse string \"%s\" as float: %v", match[5], err)
		}

		ratings[i] = Rating{
			Team:       teamRef,
			Rating:     rating,
			Points:     points,
			GoldenMean: goldenMean,
			Recent:     recent,
		}
		log.Printf("parsed team ratings: %v", ratings[i])
	}

	return &adv, ratings, nil
}
