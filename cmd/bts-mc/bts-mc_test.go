package main

import (
	"fmt"
	"io"
	"testing"
)

func TestHandler(t *testing.T) {
	week := 0
	pickers := []string{"Phil K"}
	w, req := mockRequest(pickers, &week)

	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		err := fmt.Errorf("status code %d: %s", resp.StatusCode, body)
		t.Fatal(err)
	}
}
