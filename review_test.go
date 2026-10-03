package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseRanking(t *testing.T) {
	cases := map[string][]string{
		"A is fine.\nFINAL RANKING:\n1. Answer C\n2. Answer A\n":      {"C", "A"},
		"final ranking:\n1) **B**\n2) answer aa\n3. Answer B again\n": {"B", "AA"},
		"no ranking here": nil,
	}
	for in, want := range cases {
		if got := parseRanking(in); !reflect.DeepEqual(got, want) && !(len(got) == 0 && len(want) == 0) {
			t.Errorf("%q: got %v want %v", in, got, want)
		}
	}
}

func TestPeerReviewFlow(t *testing.T) {
	sent := fakeAgents(t)
	agents := []Agent{{Name: "claude", Pane: "p1"}, {Name: "codex", Pane: "p2"}, {Name: "agy", Pane: "p3"}, {Name: "gemini", Pane: "p4"}}
	r, _ := newRun("q", agents, agents[0])
	r.PeerReview = true
	r.send()
	t0 := r.Seats[0].Sent
	st := map[string]string{"p1": "idle", "p2": "idle", "p3": "idle", "p4": "working"}
	for i := 0; i < 3; i++ {
		answer(r.Seats[i], "answer from seat "+string(rune('1'+i)))
	}
	r.poll(t0.Add(time.Second), st)
	r.poll(t0.Add(2*time.Second), st)
	if r.reviewsStarted() {
		t.Fatal("reviews started while gemini was still working")
	}
	st["p4"] = "idle" // gemini goes quiet: Silent after the grace, then reviews start with three answers
	r.poll(t0.Add(3*time.Second), st)
	r.poll(t0.Add(silentGrace+5*time.Second), st)
	if !r.reviewsStarted() || r.Reviews[3] != nil {
		t.Fatalf("reviews: started %v, silent seat reviewing %v", r.reviewsStarted(), r.Reviews[3] != nil)
	}
	for i := 0; i < 3; i++ {
		own := r.Letters[i]
		body, _ := os.ReadFile(filepath.Join(r.Reviews[i].Dir, "answers.md"))
		if strings.Contains(string(body), "BEGIN ANSWER "+own+" ") || strings.Count(string(body), "BEGIN ANSWER") != 2 {
			t.Fatalf("seat %d (letter %s) reviews the wrong set:\n%s", i, own, body)
		}
	}
	if !r.Judge.Sent.IsZero() {
		t.Fatal("judge started before the reviews settled")
	}

	// gemini answers late: letters are frozen, so it gets none and can't shift the others
	answer(r.Seats[3], "late answer")
	r.poll(t0.Add(silentGrace+6*time.Second), st)
	r.poll(t0.Add(silentGrace+7*time.Second), st)
	if r.Seats[3].State != Done || r.Letters[3] != "" {
		t.Fatalf("late seat: %v letter %q", r.Seats[3].State, r.Letters[3])
	}

	// every reviewer ranks the same answer first
	best := r.Letters[2]
	for i := 0; i < 3; i++ {
		var others []string
		for j := 0; j < 3; j++ {
			if j != i {
				others = append(others, r.Letters[j])
			}
		}
		if others[1] == best {
			others[0], others[1] = others[1], others[0]
		}
		answer(r.Reviews[i], "notes\nFINAL RANKING:\n1. Answer "+others[0]+"\n2. Answer "+others[1])
	}
	r.poll(t0.Add(silentGrace+8*time.Second), st)
	r.poll(t0.Add(silentGrace+9*time.Second), st)
	if len(r.Ranking) != 3 || r.Ranking[0].Letter != best || r.Ranking[0].Votes != 2 || r.Ranking[0].Avg != 1 {
		t.Fatalf("ranking: %+v (best should be %s)", r.Ranking, best)
	}
	if r.Judge.Sent.IsZero() {
		t.Fatal("judge did not start after the reviews")
	}
	rank, err := os.ReadFile(filepath.Join(r.Judge.Dir, "ranking.md"))
	if err != nil || !strings.Contains(string(rank), "Answer "+best) || strings.Contains(string(rank), "agy") {
		t.Fatalf("judge's ranking file: %v\n%s", err, rank)
	}
	if got := (*sent)[len(*sent)-1]; !strings.Contains(got, "ranking.md") {
		t.Fatalf("judge prompt doesn't mention the ranking: %q", got)
	}
	if !strings.Contains(r.RankingText(true), "("+r.Seats[2].Agent.Name+")") {
		t.Fatalf("revealed ranking: %s", r.RankingText(true))
	}

	again := latestRun(time.Hour, "")
	if again == nil || len(again.Ranking) != 3 || again.Letters[2] != best {
		t.Fatalf("resume lost the peer review: %+v", again)
	}
}

func TestNoReviewWithTwoAnswers(t *testing.T) {
	fakeAgents(t)
	agents := []Agent{{Name: "claude", Pane: "p1"}, {Name: "codex", Pane: "p2"}}
	r, _ := newRun("q", agents, agents[0])
	r.PeerReview = true
	r.send()
	t0 := r.Seats[0].Sent
	answer(r.Seats[0], "one")
	answer(r.Seats[1], "two")
	st := map[string]string{"p1": "idle", "p2": "idle"}
	r.poll(t0.Add(time.Second), st)
	r.poll(t0.Add(2*time.Second), st)
	if r.reviewsStarted() || r.Judge.Sent.IsZero() {
		t.Fatalf("two answers: reviews %v, judge sent %v", r.reviewsStarted(), r.Judge.Sent)
	}
}
