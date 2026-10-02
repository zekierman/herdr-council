package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SeatState int

const (
	Sending SeatState = iota
	Working
	Done
	Blocked
	Silent // the agent went idle without writing its answer
	Late
	Failed
)

func (s SeatState) String() string {
	return [...]string{"sending", "working", "done", "blocked", "no answer", "late", "failed"}[s]
}

func (s SeatState) settled() bool { return s != Sending && s != Working && s != Blocked }

const (
	seatDeadline = 10 * time.Minute
	silentGrace  = 20 * time.Second // idle this long after sending, with no file, counts as Silent
)

// Seat is one agent answering one question (or judging the answers).
type Seat struct {
	Agent     Agent
	Dir       string
	File      string
	Marker    string
	State     SeatState
	Sent      time.Time
	Finished  time.Time
	Answer    string
	Err       string
	lastSize  int64
	idleSince time.Time // herdr status flickers; only a continuous idle stretch counts as Silent
}

func (s *Seat) Elapsed(now time.Time) time.Duration {
	if s.Sent.IsZero() {
		return 0
	}
	if !s.Finished.IsZero() {
		return s.Finished.Sub(s.Sent)
	}
	return now.Sub(s.Sent)
}

// Run is one question put to the council: independent answers, then a blind verdict.
type Run struct {
	ID       string
	Dir      string
	Question string
	Seats    []*Seat
	Judge    *Seat // nil until every seat has settled; Sent is zero until the judge is prompted
	Order    []int // Order[k] is the seat shown to the judge as answer letter k
}

func letter(k int) string { return string(rune('A' + k)) }

func stateDir() string {
	if d := os.Getenv("HERDR_PLUGIN_STATE_DIR"); d != "" {
		return d
	}
	return filepath.Join(os.TempDir(), "herdr-council")
}

// newRun writes the question and gives every seat its own folder, so a slow agent
// can't read the answers already written by the others.
func newRun(question string, agents []Agent, judge Agent) (*Run, error) {
	id := time.Now().Format("20060102-150405")
	dir := filepath.Join(stateDir(), "runs", id)
	if err := os.MkdirAll(filepath.Join(dir, "judge"), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "question.md"), []byte(question+"\n"), 0o644); err != nil {
		return nil, err
	}
	r := &Run{ID: id, Dir: dir, Question: question}
	for i, a := range agents {
		seatID := fmt.Sprintf("%d-%s", i+1, a.Name)
		sd := filepath.Join(dir, seatID)
		if err := os.MkdirAll(sd, 0o755); err != nil {
			return nil, err
		}
		r.Seats = append(r.Seats, &Seat{Agent: a, Dir: sd, File: filepath.Join(sd, "answer.md"), Marker: "DONE " + id + ":" + seatID})
	}
	jd := filepath.Join(dir, "judge")
	r.Judge = &Seat{Agent: judge, Dir: jd, File: filepath.Join(jd, "verdict.md"), Marker: "DONE " + id + ":judge"}
	r.Order = rand.Perm(len(agents)) // shuffled so the judge can't infer authors from position
	return r, nil
}

// oneLine keeps prompts on a single line: newlines would submit early in agent TUIs.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func (r *Run) seatPrompt(s *Seat) string {
	q := oneLine(r.Question)
	if len([]rune(q)) > 400 {
		q = "(see the question file)"
	}
	return fmt.Sprintf("[council %s] You sit on a council of agents answering the same question independently; give your own view. "+
		"Question: %s (full text: %s). Write your complete answer as markdown to %s and make its last line exactly: %s . "+
		"Keep it under 250 words. Do not read other council folders and do not edit any other file.",
		r.ID, q, filepath.Join(r.Dir, "question.md"), s.File, s.Marker)
}

func (r *Run) judgePrompt() string {
	return fmt.Sprintf("[council %s] You are the judge of a council. Read the question at %s and the anonymous answers at %s "+
		"(authors are hidden on purpose; do not try to guess them). Write a verdict as markdown to %s with exactly these sections: "+
		"## Consensus, ## Conflicts (cite answers by letter), ## Missed or risky (gaps, errors, risks any answer overlooked), "+
		"## Final answer (one refined answer that keeps the strongest points). Under 350 words. Make its last line exactly: %s . "+
		"Do not edit any other file.",
		r.ID, filepath.Join(r.Dir, "question.md"), filepath.Join(r.Judge.Dir, "answers.md"), r.Judge.File, r.Judge.Marker)
}

// send prompts every seat. Errors stay on the seat; one failure doesn't stop the others.
func (r *Run) send() {
	for _, s := range r.Seats {
		s.Sent = time.Now()
		if err := promptAgent(s.Agent.Pane, r.seatPrompt(s)); err != nil {
			s.State, s.Err, s.Finished = Failed, err.Error(), time.Now()
			continue
		}
		s.State = Working
	}
	r.save()
}

func (r *Run) seatsSettled() bool {
	for _, s := range r.Seats {
		if !s.State.settled() {
			return false
		}
	}
	return true
}

func (r *Run) answered() int {
	n := 0
	for _, s := range r.Seats {
		if s.State == Done {
			n++
		}
	}
	return n
}

// startJudge writes the answers under shuffled letters and prompts the judge. It needs two
// answers to compare; with fewer there is nothing to judge.
func (r *Run) startJudge() {
	if r.Judge == nil || !r.Judge.Sent.IsZero() || r.answered() < 2 {
		return
	}
	var b strings.Builder
	k := 0
	for _, idx := range r.Order {
		if s := r.Seats[idx]; s.State == Done {
			fmt.Fprintf(&b, "## Answer %s\n\n%s\n\n", letter(k), s.Answer)
			k++
		}
	}
	r.Judge.Sent = time.Now()
	if err := os.WriteFile(filepath.Join(r.Judge.Dir, "answers.md"), []byte(b.String()), 0o644); err != nil {
		r.Judge.State, r.Judge.Err, r.Judge.Finished = Failed, err.Error(), time.Now()
	} else if err := promptAgent(r.Judge.Agent.Pane, r.judgePrompt()); err != nil {
		r.Judge.State, r.Judge.Err, r.Judge.Finished = Failed, err.Error(), time.Now()
	} else {
		r.Judge.State = Working
	}
	r.save()
}

// Reveal maps the judge's letters back to agents, in the order the judge saw them.
func (r *Run) Reveal() []string {
	var out []string
	k := 0
	for _, idx := range r.Order {
		if s := r.Seats[idx]; s.State == Done {
			out = append(out, letter(k)+" = "+s.Agent.Name)
			k++
		}
	}
	return out
}

// poll advances seats from their answer files and herdr status, then hands over to the judge.
func (r *Run) poll(now time.Time, status map[string]string) {
	for _, s := range r.Seats {
		pollSeat(s, now, status)
	}
	if r.seatsSettled() {
		r.startJudge()
	}
	if r.Judge != nil && !r.Judge.Sent.IsZero() {
		pollSeat(r.Judge, now, status)
	}
}

func pollSeat(s *Seat, now time.Time, status map[string]string) {
	if s.State == Done || s.State == Failed || s.Sent.IsZero() {
		return
	}
	if fi, err := os.Stat(s.File); err == nil {
		size := fi.Size()
		stable := size > 0 && size == s.lastSize
		s.lastSize = size
		if stable {
			if body, ok := readAnswer(s.File, s.Marker); ok {
				s.Answer, s.State, s.Finished = body, Done, now
				return
			}
		}
	}
	st := status[s.Agent.Pane]
	if st == "idle" || st == "done" {
		if s.idleSince.IsZero() {
			s.idleSince = now
		}
	} else {
		s.idleSince = time.Time{}
	}
	switch {
	case st == "blocked":
		s.State = Blocked
	case now.Sub(s.Sent) > seatDeadline:
		s.State = Late
	case !s.idleSince.IsZero() && now.Sub(s.idleSince) > silentGrace:
		s.State = Silent
	default:
		s.State = Working
	}
}

// finished: every seat settled and the judge done, failed, or not needed.
func (r *Run) finished() bool {
	if !r.seatsSettled() {
		return false
	}
	if r.Judge == nil || r.answered() < 2 {
		return true
	}
	return !r.Judge.Sent.IsZero() && r.Judge.State.settled()
}

// meta is what a closed council needs to pick a run back up: the answers live in the files.
type meta struct {
	ID       string     `json:"id"`
	Question string     `json:"question"`
	Order    []int      `json:"order"`
	Seats    []seatMeta `json:"seats"`
	Judge    seatMeta   `json:"judge"`
}

type seatMeta struct {
	Name   string    `json:"name"`
	Pane   string    `json:"pane"`
	File   string    `json:"file"`
	Marker string    `json:"marker"`
	Sent   time.Time `json:"sent"`
	Err    string    `json:"error,omitempty"`
}

func toMeta(s *Seat) seatMeta {
	return seatMeta{s.Agent.Name, s.Agent.Pane, s.File, s.Marker, s.Sent, s.Err}
}

func (r *Run) save() {
	m := meta{ID: r.ID, Question: r.Question, Order: r.Order, Judge: toMeta(r.Judge)}
	for _, s := range r.Seats {
		m.Seats = append(m.Seats, toMeta(s))
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(filepath.Join(r.Dir, "run.json"), b, 0o644)
}

func fromMeta(sm seatMeta) *Seat {
	s := &Seat{Agent: Agent{Name: sm.Name, Pane: sm.Pane}, Dir: filepath.Dir(sm.File), File: sm.File, Marker: sm.Marker, Sent: sm.Sent, State: Working}
	switch {
	case sm.Err != "":
		s.State, s.Err, s.Finished = Failed, sm.Err, sm.Sent
	case sm.Sent.IsZero():
		s.State = Sending
	default:
		if body, ok := readAnswer(sm.File, sm.Marker); ok {
			fi, _ := os.Stat(sm.File)
			s.Answer, s.State, s.Finished = body, Done, fi.ModTime()
		}
	}
	return s
}

// latestRun reopens the most recent run if it started within maxAge, so closing the popup
// never loses a question: the agents keep writing and the next open shows their answers.
func latestRun(maxAge time.Duration) *Run {
	paths, _ := filepath.Glob(filepath.Join(stateDir(), "runs", "*", "run.json"))
	if len(paths) == 0 {
		return nil
	}
	sort.Strings(paths)
	path := paths[len(paths)-1]
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m meta
	if json.Unmarshal(b, &m) != nil || len(m.Seats) == 0 || time.Since(m.Seats[0].Sent) > maxAge {
		return nil
	}
	r := &Run{ID: m.ID, Dir: filepath.Dir(path), Question: m.Question, Order: m.Order, Judge: fromMeta(m.Judge)}
	for _, sm := range m.Seats {
		r.Seats = append(r.Seats, fromMeta(sm))
	}
	if len(r.Order) != len(r.Seats) {
		r.Order = rand.Perm(len(r.Seats))
	}
	return r
}

// readAnswer accepts the file only when its last non-empty line is the seat's marker.
func readAnswer(path, marker string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 || strings.TrimSpace(lines[len(lines)-1]) != marker {
		return "", false
	}
	return strings.TrimSpace(strings.Join(lines[:len(lines)-1], "\n")), true
}
