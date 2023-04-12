// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	_, err := g.Next()
	if err != EndOfList {
		t.Fatalf("got %v, want %v items", err, EndOfList)
	}
}
func ptr(s string) *string {
	return &s
}
