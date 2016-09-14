package main

import "fmt"

type remainingMap map[string][]string

func (r *remainingMap) TrimUsers(w int) (map[string]bool, error) {
	// Users can have up to 1 more team than weeks availible, but not fewer
	ddusers := make(map[string]bool)
	expected := 14 - w
	for user, rem := range *r {
		l := len(rem)
		if l < expected {
			return nil, fmt.Errorf("too few remaining teams : expected %v, got %v", expected, l)
		}
		if l == expected+1 {
			ddusers[user] = true
		} else if l > expected+1 {
			delete(*r, user)
		} else {
			ddusers[user] = false
		}
	}

	return ddusers, nil
}

func (r *remainingMap) Users() []string {
	keys := make([]string, len(*r))

	i := 0
	for k := range *r {
		keys[i] = k
		i++
	}

	return keys
}