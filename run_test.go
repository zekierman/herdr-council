package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadAnswer(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.md")
	cases := []struct {
		body string
		ok   bool
		want string
	}{
		{"Use a cookie.\nDONE r1:1-claude\n", true, "Use a cookie."},
		{"Use a cookie.\r\nDONE r1:1-claude\r\n\r\n", true, "Use a cookie."},
		{"Use a cookie.\nDONE r1:2-codex\n", false, ""},
		{"DONE r1:1-claude\nstill writing", false, ""},
		{"", false, ""},
	}
	for _, c := range cases {
		os.WriteFile(p, []byte(c.body), 0o644)
		got, ok := readAnswer(p, "DONE r1:1-claude")
		if ok != c.ok || got != c.want {
			t.Errorf("%q: got (%q, %v), want (%q, %v)", c.body, got, ok, c.want, c.ok)
		}
	}
}

// fakeAgents stubs promptAgent so tests never touch herdr.
func fakeAgents(t *testing.T) *[]string {
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	var sent []string
	old := promptAgent
	promptAgent = func(pane, text string) error { sent = append(sent, pane+": "+text); return nil }
	t.Cleanup(func() { promptAgent = old })
	return &sent
}

func answer(s *Seat, body string) { os.WriteFile(s.File, []byte(body+"\n"+s.Marker+"\n"), 0o644) }

func TestCouncilFlow(t *testing.T) {
	sent := fakeAgents(t)
	agents := []Agent{{Name: "claude", Pane: "p1"}, {Name: "codex", Pane: "p2"}, {Name: "agy", Pane: "p3"}}
	r, err := newRun("Where should a refresh token live?", agents, agents[0])
	if err != nil {
		t.Fatal(err)
	}
	r.send()
	if len(*sent) != 3 {
		t.Fatalf("seats prompted: %d", len(*sent))
	}
	t0 := r.Seats[0].Sent
	status := map[string]string{"p1": "working", "p2": "blocked", "p3": "working"}
	answer(r.Seats[0], "Keychain, first view.")
	answer(r.Seats[2], "Keychain, second view.")
	r.poll(t0.Add(time.Second), status) // files seen once: not yet stable
	r.poll(t0.Add(2*time.Second), status)
	if r.Seats[0].State != Done || r.Seats[1].State != Blocked || r.Seats[2].State != Done {
		t.Fatalf("states: %v %v %v", r.Seats[0].State, r.Seats[1].State, r.Seats[2].State)
	}
	if !r.Judge.Sent.IsZero() {
		t.Fatal("judge started while a seat was still blocked")
	}

	status["p2"] = "idle" // codex goes quiet and never writes
	r.poll(t0.Add(3*time.Second), status)
	if r.Seats[1].State == Silent {
		t.Fatal("silent on the first idle sighting")
	}
	status["p2"] = "working" // a flicker back to working resets the idle clock
	r.poll(t0.Add(4*time.Second), status)
	status["p2"] = "idle"
	r.poll(t0.Add(5*time.Second), status)
	r.poll(t0.Add(silentGrace+4*time.Second), status)
	if r.Seats[1].State == Silent {
		t.Fatal("silent although the idle stretch was interrupted")
	}
	r.poll(t0.Add(silentGrace+6*time.Second), status) // idle without a break past the grace: Silent, and the judge goes ahead
	if r.Seats[1].State != Silent || r.Judge.Sent.IsZero() {
		t.Fatalf("codex %v, judge sent %v", r.Seats[1].State, r.Judge.Sent)
	}
	blind, _ := os.ReadFile(filepath.Join(r.Judge.Dir, "answers.md"))
	for _, name := range []string{"claude", "codex", "agy"} {
		if strings.Contains(strings.ToLower(string(blind)), name) {
			t.Fatalf("judge input names an author: %s", blind)
		}
	}
	if !strings.Contains(string(blind), "## Answer A") || !strings.Contains(string(blind), "## Answer B") || strings.Contains(string(blind), "## Answer C") {
		t.Fatalf("judge input letters: %s", blind)
	}
	if got := (*sent)[len(*sent)-1]; !strings.HasPrefix(got, "p1: ") || !strings.Contains(got, "judge") {
		t.Fatalf("judge prompt went to %q", got)
	}
	if rev := r.Reveal(); len(rev) != 2 {
		t.Fatalf("reveal: %v", rev)
	}
	if r.finished() {
		t.Fatal("finished before the verdict")
	}
	answer(r.Judge, "## Consensus\nKeychain.")
	r.poll(t0.Add(silentGrace+7*time.Second), status)
	r.poll(t0.Add(silentGrace+8*time.Second), status)
	if r.Judge.State != Done || !r.finished() {
		t.Fatalf("judge %v finished %v", r.Judge.State, r.finished())
	}
}

func TestNoJudgeWithOneAnswer(t *testing.T) {
	fakeAgents(t)
	agents := []Agent{{Name: "claude", Pane: "p1"}, {Name: "codex", Pane: "p2"}}
	r, _ := newRun("q?", agents, agents[0])
	r.send()
	t0 := r.Seats[0].Sent
	answer(r.Seats[0], "only me")
	status := map[string]string{"p1": "idle", "p2": "idle"}
	r.poll(t0.Add(time.Second), status)
	r.poll(t0.Add(silentGrace+2*time.Second), status)
	if !r.Judge.Sent.IsZero() || !r.finished() {
		t.Fatalf("judge sent %v, finished %v", r.Judge.Sent, r.finished())
	}
}

func TestLatestRunResumes(t *testing.T) {
	fakeAgents(t)
	agents := []Agent{{Name: "claude", Pane: "p1"}, {Name: "codex", Pane: "p2"}}
	r, _ := newRun("q?", agents, agents[1])
	r.send()
	answer(r.Seats[1], "later answer")
	got := latestRun(time.Hour)
	if got == nil || got.Question != "q?" || len(got.Seats) != 2 || got.Judge.Agent.Name != "codex" {
		t.Fatalf("resume: %+v", got)
	}
	if got.Seats[0].State != Working || got.Seats[1].State != Done || got.Seats[1].Answer != "later answer" {
		t.Fatalf("states: %v %v %q", got.Seats[0].State, got.Seats[1].State, got.Seats[1].Answer)
	}
	if got.Judge.State != Sending || len(got.Order) != 2 {
		t.Fatalf("judge %v order %v", got.Judge.State, got.Order)
	}
	if latestRun(-time.Second) != nil {
		t.Fatal("an old run should not resume")
	}
}
