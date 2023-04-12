package model

import (
	"testing"

	"github.com/google/go-github/v51/github"
)

func TestGenerator(t *testing.T) {
	r := github.Response{}
	g := ListGenerator[string]{
		Retrieve: func(github.ListOptions) ([]*string, *github.Response, error) {
			r.NextPage++
			r.LastPage = 2
			if r.NextPage == 2 {
				r.NextPage = 0
			}
			return []*string{ptr("one"), ptr("two"), ptr("three")}, &r, nil
		},
	}
	want := 6
	got := 0
	for g.HasNext() {
		g.Next()
		got++
	}
	if got != want {
		t.Fatalf("got %v, want %v items", got, want)
	}
}

func TestGeneratorEmptyList(t *testing.T) {
	r := github.Response{}
	g := ListGenerator[string]{
		Retrieve: func(github.ListOptions) ([]*string, *github.Response, error) {
			r.NextPage = 0
			r.LastPage = 0
			return []*string{}, &r, nil
		},
	}
	want := 0
	got := 0
	for g.HasNext() {
		g.Next()
		got++
	}
	if got != want {
		t.Fatalf("got %v, want %v items", got, want)
	}
}
func ptr(s string) *string {
	return &s
}
