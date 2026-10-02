// herdr-council: put one question to every coding agent in a herdr workspace, then let a
// judge agent weigh their answers blind and write a single verdict.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
)

var (
	dim     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6e7681"))
	text    = lipgloss.NewStyle().Foreground(lipgloss.Color("#c9d1d9"))
	bold    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e6edf3"))
	accent  = lipgloss.NewStyle().Foreground(lipgloss.Color("#9cc7ff"))
	good    = lipgloss.NewStyle().Foreground(lipgloss.Color("#7ee787"))
	warn    = lipgloss.NewStyle().Foreground(lipgloss.Color("#e3b341"))
	bad     = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff7b72"))
	button  = lipgloss.NewStyle().Foreground(lipgloss.Color("#e6edf3")).Background(lipgloss.Color("#30363d")).Padding(0, 1)
	primary = lipgloss.NewStyle().Foreground(lipgloss.Color("#0d1117")).Background(lipgloss.Color("#9cc7ff")).Bold(true).Padding(0, 1)
	spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

type phase int

const (
	asking phase = iota
	watching
)

type tickMsg time.Time
type agentsMsg struct {
	agents []Agent
	err    error
}

// zone is a clickable span on screen: row y, columns [x0, x1).
type zone struct {
	y, x0, x1 int
	act       string
}

type model struct {
	phase     phase
	workspace string
	w, h      int
	frame     int

	input    textarea.Model
	agents   []Agent
	picked   map[string]bool
	judge    int // index into agents
	onSeats  bool
	cursor   int
	agentErr string

	run        *Run
	seat       int // len(run.Seats) selects the verdict
	sawVerdict bool
	answer     viewport.Model
	flash      string
}

func newModel(workspace string) model {
	ta := textarea.New()
	ta.Placeholder = "What should the council weigh in on?"
	ta.Prompt = "┃ "
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.Focus()
	return model{workspace: workspace, input: ta, picked: map[string]bool{}, judge: -1, answer: viewport.New()}
}

func fetchAgents(ws string) tea.Cmd {
	return func() tea.Msg {
		a, err := listAgents(ws)
		return agentsMsg{a, err}
	}
}

func tick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Init() tea.Cmd { return tea.Batch(fetchAgents(m.workspace), tick()) }

func (m *model) resize() {
	m.input.SetWidth(max(20, m.w-4))
	m.answer.SetWidth(max(20, m.w-2))
	m.answer.SetHeight(max(3, m.h-10))
	m.refreshAnswer()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.resize()
		return m, nil

	case agentsMsg:
		if msg.err != nil {
			m.agentErr = msg.err.Error()
			return m, nil
		}
		m.agentErr = ""
		first := len(m.agents) == 0
		m.agents = msg.agents
		if first {
			for i, a := range m.agents {
				m.picked[a.Pane] = a.Status == "idle" || a.Status == "done"
				if a.Name == "claude" && m.judge < 0 {
					m.judge = i
				}
			}
		}
		if m.judge < 0 || m.judge >= len(m.agents) {
			m.judge = 0
		}
		if m.run != nil {
			status := map[string]string{}
			for _, a := range m.agents {
				status[a.Pane] = a.Status
			}
			m.run.poll(time.Now(), status)
			if m.run.Judge != nil && m.run.Judge.State == Done && !m.sawVerdict {
				m.seat, m.sawVerdict = len(m.run.Seats), true // the verdict is the point: show it when it lands
			}
			m.refreshAnswer()
		}
		return m, nil

	case tickMsg:
		m.frame++
		cmds := []tea.Cmd{tick()}
		if m.frame%2 == 0 { // herdr status once a second
			cmds = append(cmds, fetchAgents(m.workspace))
		}
		return m, tea.Batch(cmds...)

	case tea.MouseClickMsg:
		mo := msg.Mouse()
		if mo.Button != tea.MouseLeft {
			return m, nil
		}
		for _, z := range m.layout().zones {
			if mo.Y == z.y && mo.X >= z.x0 && mo.X < z.x1 {
				return m.do(z.act)
			}
		}
		return m, nil

	case tea.MouseWheelMsg:
		if m.phase == watching {
			var cmd tea.Cmd
			m.answer, cmd = m.answer.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.phase == asking {
			return m.updateAsking(msg)
		}
		return m.updateWatching(msg)
	}
	if m.phase == asking && !m.onSeats {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

// do runs one named action: shared by keys and mouse clicks.
func (m model) do(act string) (tea.Model, tea.Cmd) {
	switch {
	case act == "close":
		return m, tea.Quit
	case act == "ask":
		return m.ask()
	case act == "input":
		m.onSeats = false
		return m, m.input.Focus()
	case strings.HasPrefix(act, "toggle:"):
		var i int
		fmt.Sscanf(act, "toggle:%d", &i)
		if i < len(m.agents) {
			p := m.agents[i].Pane
			m.picked[p] = !m.picked[p]
			m.cursor = i
		}
	case act == "judge":
		if len(m.agents) > 0 {
			m.judge = (m.judge + 1) % len(m.agents)
		}
	case strings.HasPrefix(act, "seat:"):
		fmt.Sscanf(act, "seat:%d", &m.seat)
	case act == "copy":
		if body := m.currentText(); body != "" && clipboard.WriteAll(body) == nil {
			m.flash = "copied"
		}
		return m, nil
	case act == "goto":
		if a := m.currentAgent(); a != nil {
			if err := focusAgent(a.Pane); err != nil {
				m.flash = err.Error()
				return m, nil
			}
			return m, tea.Quit
		}
	case act == "judge-now":
		if m.run.answered() >= 2 && m.run.Judge.Sent.IsZero() {
			m.run.startJudge()
			m.seat = len(m.run.Seats)
		}
	case act == "new":
		nm := newModel(m.workspace)
		nm.w, nm.h, nm.agents, nm.picked, nm.judge = m.w, m.h, m.agents, m.picked, m.judge
		nm.resize()
		return nm, nm.input.Focus()
	}
	m.flash = ""
	m.refreshAnswer()
	return m, nil
}

func (m model) updateAsking(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, tea.Quit
	case "tab":
		m.onSeats = !m.onSeats
		if m.onSeats {
			m.input.Blur()
			return m, nil
		}
		return m, m.input.Focus()
	case "enter":
		return m.ask()
	}
	if m.onSeats {
		switch msg.String() {
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
		case "down", "j":
			m.cursor = min(len(m.agents)-1, m.cursor+1)
		case "space", "x":
			return m.do(fmt.Sprintf("toggle:%d", m.cursor))
		case "J":
			return m.do("judge")
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) ask() (tea.Model, tea.Cmd) {
	q := strings.TrimSpace(m.input.Value())
	var seats []Agent
	for _, a := range m.agents {
		if m.picked[a.Pane] {
			seats = append(seats, a)
		}
	}
	if q == "" || len(seats) == 0 || m.judge < 0 || m.judge >= len(m.agents) {
		m.flash = "write a question and pick at least one seat"
		return m, nil
	}
	r, err := newRun(q, seats, m.agents[m.judge])
	if err != nil {
		m.flash = err.Error()
		return m, nil
	}
	r.send()
	m.run, m.seat, m.phase, m.flash, m.sawVerdict = r, 0, watching, "", false
	m.refreshAnswer()
	return m, nil
}

func (m model) updateWatching(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	n := len(m.run.Seats) + 1
	switch k := msg.String(); k {
	case "q", "esc":
		return m.do("close")
	case "tab", "right", "l":
		m.seat = (m.seat + 1) % n
	case "shift+tab", "left", "h":
		m.seat = (m.seat + n - 1) % n
	case "v":
		m.seat = len(m.run.Seats)
	case "c":
		return m.do("copy")
	case "f":
		return m.do("goto")
	case "j":
		return m.do("judge-now")
	case "n":
		return m.do("new")
	default:
		if len(k) == 1 && k[0] >= '1' && k[0] <= '9' && int(k[0]-'1') < len(m.run.Seats) {
			m.seat = int(k[0] - '1')
		} else {
			var cmd tea.Cmd
			m.answer, cmd = m.answer.Update(msg)
			return m, cmd
		}
	}
	m.flash = ""
	m.refreshAnswer()
	return m, nil
}

func (m model) onVerdict() bool { return m.run != nil && m.seat >= len(m.run.Seats) }

func (m model) currentSeat() *Seat {
	if m.run == nil {
		return nil
	}
	if m.onVerdict() {
		return m.run.Judge
	}
	return m.run.Seats[m.seat]
}

func (m model) currentAgent() *Agent {
	if s := m.currentSeat(); s != nil {
		return &s.Agent
	}
	return nil
}

func (m model) currentText() string {
	s := m.currentSeat()
	if s == nil || s.State != Done {
		return ""
	}
	if m.onVerdict() {
		return s.Answer + "\n\n" + strings.Join(m.run.Reveal(), ", ")
	}
	return s.Answer
}

func (m *model) refreshAnswer() {
	if m.run == nil {
		return
	}
	wrap := text.Width(max(20, m.w-4))
	if m.onVerdict() {
		m.answer.SetContent(m.verdictBody(wrap))
		return
	}
	s := m.run.Seats[m.seat]
	var body string
	switch s.State {
	case Done:
		body = wrap.Render(s.Answer)
	case Working, Sending:
		body = dim.Render(s.Agent.Name + " is thinking on its own; the others' answers stay hidden from it.")
	case Blocked:
		body = warn.Render(s.Agent.Name+" is waiting for you, probably a permission prompt.") + "\n" + dim.Render("click Go to agent (or press f) to answer it")
	case Silent:
		body = warn.Render(s.Agent.Name+" went idle without writing its answer.") + "\n" + dim.Render("click Go to agent (or press f) to look at its pane")
	case Late:
		body = warn.Render(s.Agent.Name+" has not answered in 10 minutes.") + "\n" + dim.Render("click Go to agent (or press f) to look at its pane")
	case Failed:
		body = bad.Render("could not reach " + s.Agent.Name + ": " + s.Err)
	}
	m.answer.SetContent(body)
}

func (m model) verdictBody(wrap lipgloss.Style) string {
	r, j := m.run, m.run.Judge
	switch {
	case j.State == Done:
		return wrap.Render(j.Answer)
	case j.State == Failed:
		return bad.Render("the judge could not be reached: " + j.Err)
	case !j.Sent.IsZero() && j.State == Blocked:
		return warn.Render(j.Agent.Name + " is waiting for you, probably a permission prompt.")
	case !j.Sent.IsZero() && j.State.settled():
		return warn.Render(j.Agent.Name + " went quiet without writing the verdict.")
	case !j.Sent.IsZero():
		return dim.Render(fmt.Sprintf("%s is weighing %d answers blind: it sees them as A, B, C… without names.", j.Agent.Name, r.answered()))
	case r.seatsSettled() && r.answered() < 2:
		return dim.Render("A verdict needs at least two answers to compare.")
	case r.answered() >= 2:
		return dim.Render(fmt.Sprintf("%s will judge once every seat is done. Click Judge now (or press j) to start with the %d answers in.", j.Agent.Name, r.answered()))
	default:
		return dim.Render(j.Agent.Name + " will judge the answers blind once the seats are done.")
	}
}

func badge(s *Seat, frame int, now time.Time) string {
	el := fmt.Sprintf("%ds", int(s.Elapsed(now).Seconds()))
	switch s.State {
	case Done:
		return good.Render("✓") + " " + dim.Render(el)
	case Working:
		return accent.Render(spinner[frame%len(spinner)]) + " " + dim.Render(el)
	case Sending:
		return dim.Render("·")
	case Blocked:
		return warn.Render("! waiting")
	case Silent:
		return warn.Render("? no answer")
	case Late:
		return warn.Render("… late")
	default:
		return bad.Render("× failed")
	}
}

type screen struct {
	lines []string
	zones []zone
}

// add appends one line made of segments; segments with an action become clickable.
func (s *screen) add(segs ...[2]string) {
	x, y := 0, len(s.lines)
	var b strings.Builder
	for _, sg := range segs {
		w := lipgloss.Width(sg[0])
		if sg[1] != "" {
			s.zones = append(s.zones, zone{y, x, x + w, sg[1]})
		}
		b.WriteString(sg[0])
		x += w
	}
	s.lines = append(s.lines, b.String())
}

func seg(s string) [2]string         { return [2]string{s, ""} }
func act(s, action string) [2]string { return [2]string{s, action} }
func (s *screen) blank()             { s.lines = append(s.lines, "") }

// raw appends a multi-line block; with an action, every row of it is clickable.
func (s *screen) raw(block, action string) {
	for _, l := range strings.Split(block, "\n") {
		if action != "" {
			s.zones = append(s.zones, zone{len(s.lines), 0, lipgloss.Width(l), action})
		}
		s.lines = append(s.lines, l)
	}
}

// layout builds the screen and its click zones together, so clicks always match what's drawn.
func (m model) layout() screen {
	var sc screen
	width := max(20, m.w)
	rule := dim.Render(" " + strings.Repeat("─", width-2))
	if m.phase == asking {
		sc.add(seg(" "+bold.Render("COUNCIL")), seg(dim.Render(shorten("  /  one question, independent answers, blind verdict", width-9))))
		if width >= 70 && m.h >= 28 {
			for _, line := range councilArt {
				sc.add(seg(accent.Render(line)))
			}
		} else {
			sc.add(seg(" " + dim.Render("Ask once. Compare independent views.")))
		}
		sc.blank()
		sc.add(seg(" "+accent.Render("01")+"  "+bold.Render("Ask")), seg(dim.Render("  /  Write the question everyone will answer.")))
		sc.raw(indent(m.input.View()), "input")
		sc.blank()
		sc.add(seg(" " + accent.Render("02") + "  " + bold.Render("Seats")))
		sc.add(seg("     " + dim.Render(shorten("Each selected agent answers alone; no seat sees another answer.", width-5))))
		if m.agentErr != "" {
			sc.add(seg(" " + bad.Render(shorten(m.agentErr, width-2))))
		}
		if len(m.agents) == 0 && m.agentErr == "" {
			sc.add(seg(dim.Render("   no agents in this workspace yet")))
		}
		for i, a := range m.agents {
			box := dim.Render("[ ]")
			if m.picked[a.Pane] {
				box = accent.Render("[x]")
			}
			pre := "   "
			if m.onSeats && i == m.cursor {
				pre = accent.Render(" › ")
			}
			sc.add(act(pre+box+" "+fmt.Sprintf("%-12s", shorten(a.Name, 12))+" "+dim.Render(shorten(a.Status, 13)), fmt.Sprintf("toggle:%d", i)))
		}
		sc.blank()
		sc.add(seg(" " + accent.Render("03") + "  " + bold.Render("Judge")))
		sc.add(seg("     " + dim.Render(shorten("Reads answers as A/B/C, then writes a verdict without names.", width-5))))
		judge := "none"
		if m.judge >= 0 && m.judge < len(m.agents) {
			judge = shorten(m.agents[m.judge].Name, 14)
		}
		sc.add(seg("     "), act(button.Render("‹ "+judge+" ›"), "judge"))
		sc.blank()
		sc.add(seg(" "), act(primary.Render("Ask the council"), "ask"), seg("  "), act(button.Render("Close"), "close"))
		if m.flash != "" {
			sc.add(seg(" " + warn.Render(shorten(m.flash, width-2))))
		}
		for len(sc.lines) < m.h-1 {
			sc.blank()
		}
		sc.add(seg(dim.Render(" enter ask · tab seats · space toggle · J judge · esc close")))
		return sc
	}

	now := time.Now()
	r := m.run
	head := fmt.Sprintf("  /  %d of %d seats answered  /  judge: %s", r.answered(), len(r.Seats), r.Judge.Agent.Name)
	sc.add(seg(" "+bold.Render("COUNCIL")), seg(dim.Render(shorten(head, width-10))))
	q := oneLine(r.Question)
	sc.add(seg(dim.Render(" Q  ")), seg(text.Render(shorten(q, width-5))))
	sc.add(seg(dim.Render(shorten(" Seats answer independently. The judge sees anonymous letters.", width))))
	for _, row := range m.tabRows(now, width) {
		sc.add(row...)
	}
	sc.add(seg(rule))
	if m.onVerdict() {
		sc.add(seg(" " + accent.Render(".----< VERDICT >----.") + "  " + bold.Render(verdictState(r.Judge))))
		if r.Judge.State == Done {
			sc.add(seg(" " + good.Render("REVEALED") + "  " + text.Render(shorten(strings.Join(r.Reveal(), "  ·  "), width-12))))
		}
	} else {
		sc.add(seg(" "+bold.Render("ANSWER")), seg(dim.Render("  /  "+r.Seats[m.seat].Agent.Name)))
	}
	// Reserve the footer first, so wrapped tabs and the reveal never push buttons off-screen.
	available := max(3, m.h-len(sc.lines)-3)
	vp := m.answer
	vp.SetHeight(available)
	sc.raw(indent(vp.View()), "")
	sc.add(seg(rule))
	btns := [][2]string{seg(" "), act(button.Render("Copy"), "copy"), seg(" "), act(button.Render("Go to agent"), "goto")}
	if r.answered() >= 2 && r.Judge.Sent.IsZero() {
		btns = append(btns, seg(" "), act(primary.Render("Judge now"), "judge-now"))
	}
	btns = append(btns, seg(" "), act(button.Render("New question"), "new"), seg(" "), act(button.Render("Close"), "close"))
	sc.add(btns...)
	if m.flash != "" {
		sc.add(seg(" " + warn.Render(shorten(m.flash, width-2))))
	} else {
		sc.add(seg(dim.Render(shorten(" 1-9 seat · v verdict · ↑↓ scroll · c copy · f agent · j judge now · n new · q close", width))))
	}
	return sc
}

func indent(block string) string {
	ls := strings.Split(block, "\n")
	for i := range ls {
		ls[i] = " " + ls[i]
	}
	return strings.Join(ls, "\n")
}

func (m model) View() tea.View {
	v := tea.NewView(strings.Join(m.layout().lines, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// headless asks without the UI: send, poll until the seats and the judge settle, print everything.
func headless(question, seatList, judgeName, ws string) int {
	if ws == "" {
		ws = currentWorkspace()
	}
	agents, err := listAgents(ws)
	if err != nil {
		fmt.Fprintln(os.Stderr, "council:", err)
		return 1
	}
	want := map[string]bool{}
	for _, n := range strings.Split(seatList, ",") {
		if n = strings.TrimSpace(n); n != "" {
			want[n] = true
		}
	}
	var picked []Agent
	var judge *Agent
	for i, a := range agents {
		if (len(want) == 0 && (a.Status == "idle" || a.Status == "done")) || want[a.Name] {
			picked = append(picked, a)
		}
		if a.Name == judgeName {
			judge = &agents[i]
		}
	}
	if len(picked) == 0 {
		fmt.Fprintln(os.Stderr, "council: no seats")
		return 1
	}
	if judge == nil {
		judge = &picked[0]
	}
	r, err := newRun(question, picked, *judge)
	if err != nil {
		fmt.Fprintln(os.Stderr, "council:", err)
		return 1
	}
	r.send()
	for !r.finished() {
		time.Sleep(time.Second)
		status := map[string]string{}
		if as, err := listAgents(ws); err == nil {
			for _, a := range as {
				status[a.Pane] = a.Status
			}
		}
		r.poll(time.Now(), status)
	}
	now := time.Now()
	for _, s := range r.Seats {
		fmt.Printf("== %s (%s, %s)\n%s\n\n", s.Agent.Name, s.State, s.Elapsed(now).Round(time.Second), s.Answer)
	}
	if !r.Judge.Sent.IsZero() {
		fmt.Printf("== verdict by %s (%s, %s)\n%s\n\nrevealed: %s\n\n", r.Judge.Agent.Name, r.Judge.State, r.Judge.Elapsed(now).Round(time.Second), r.Judge.Answer, strings.Join(r.Reveal(), " · "))
	}
	fmt.Println("run:", r.Dir)
	return 0
}

func main() {
	ws := flag.String("workspace", "", "herdr workspace id (default: the current one)")
	open := flag.Bool("open", false, "open the council over the focused pane and exit")
	placement := flag.String("placement", "", "with --open: popup (default) or tab, a full-size tab that keeps its place")
	ask := flag.String("ask", "", "ask without the UI and print the answers and the verdict (for scripts and testing)")
	seats := flag.String("seats", "", "comma-separated agent names to ask with --ask (default: all idle)")
	judge := flag.String("judge", "claude", "agent name that judges with --ask (default: claude, else the first seat)")
	flag.Parse()
	if *ask != "" {
		os.Exit(headless(*ask, *seats, *judge, *ws))
	}
	if *open {
		args := []string{"plugin", "pane", "open", "--plugin", "herdr-council", "--entrypoint", "council"}
		if *placement != "" {
			args = append(args, "--placement", *placement)
		}
		if _, err := herdr(args...); err != nil {
			fmt.Fprintln(os.Stderr, "council:", err)
			os.Exit(1)
		}
		return
	}
	w := *ws
	if w == "" {
		w = currentWorkspace()
	}
	m := newModel(w)
	if r := latestRun(30 * time.Minute); r != nil {
		m.run, m.phase = r, watching
		m.sawVerdict = r.Judge != nil && r.Judge.State == Done
	}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "council:", err)
		os.Exit(1)
	}
}
