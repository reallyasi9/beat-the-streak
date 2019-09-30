package sagarin

import (
	"context"
	"encoding/json"
	"testing"
)

func TestScrapeSagarin(t *testing.T) {
	ctx := context.Background()
	testMsg := ScrapeMessage{
		SagarinURL: "https://sagarin.com/sports/cfsend.htm",
	}
	testBytes, _ := json.Marshal(testMsg)
	m := PubSubMessage{Data: testBytes}

	err := ScrapeSagarin(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
}
