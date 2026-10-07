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

// UI indirection lets copy tests avoid modifying the user's clipboard.
var writeClipboard = clipboard.WriteAll

type phase int

const (
	asking phase = iota
	watching
	welcoming
)

type askFocus int

const (
	focusQuestion askFocus = iota
	focusSeats
	focusJudge
	focusPeer
	focusAsk
	focusClose
	focusCount
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

	input       textarea.Model
	agents      []Agent
	picked      map[string]bool
	judge       int // index into agents
	focus       askFocus
	cursor      int
	seatTop     int
	confirmAsk  bool
	peerReview  bool
	peerChosen  bool
	judgeOpen   bool
	judgeCursor int
	judgeTop    int
	agentErr    string

	run            *Run
	seat           int // len(run.Seats) selects the verdict
	sawVerdict     bool
	showReview     bool
	answer         viewport.Model
	flash          string
	help           bool
	settings       Settings
	shortcut       string
	preferenceNote string
	welcomeFocus   int
	settingsOpen   bool
	settingsFocus  int
	settingsReturn string
	page           string
	runs           []*Run
	runCursor      int
	runTop         int
	oldRun         bool // History is read-only: poll can start reviews and the judge.
}

func newModel(workspace string) model {
	ta := textarea.New()
	ta.Placeholder = "What should the council weigh in on?"
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	styles := textarea.DefaultDarkStyles()
	styles.Focused.Text, styles.Blurred.Text = text, text
	styles.Focused.CursorLine, styles.Blurred.CursorLine = text, text
	styles.Focused.Placeholder, styles.Blurred.Placeholder = dim, dim
	styles.Focused.EndOfBuffer, styles.Blurred.EndOfBuffer = dim, dim
	styles.Cursor.Color = lipgloss.Color(accentHex)
	ta.SetStyles(styles)
	ta.SetHeight(3)
	ta.Focus()
	return model{workspace: workspace, input: ta, picked: map[string]bool{}, judge: -1, focus: focusQuestion, peerReview: true, answer: viewport.New(), page: "ask"}
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
	contentWidth, _ := m.contentSize()
	m.input.SetWidth(max(20, contentWidth-8))
	m.refreshAnswer()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "?":
			m.help = !m.help
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.help {
				m.help = false
				return m, nil
			}
		}
		if m.help {
			return m, nil
		}
		if m.phase != welcoming && !m.judgeOpen {
			switch key.String() {
			case "ctrl+left":
				return m.switchPage(-1)
			case "ctrl+right":
				return m.switchPage(1)
			}
		}
	}
	if m.help {
		if click, ok := msg.(tea.MouseClickMsg); ok {
			mo := click.Mouse()
			if mo.Button == tea.MouseLeft {
				for _, z := range m.layout().zones {
					if z.act == "help-close" && mo.Y == z.y && mo.X >= z.x0 && mo.X < z.x1 {
						m.help = false
						break
					}
				}
			}
			return m, nil
		}
	}
	if m.settingsOpen {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			return m.updateSettings(key)
		}
		if click, ok := msg.(tea.MouseClickMsg); ok {
			mo := click.Mouse()
			if mo.Button == tea.MouseLeft {
				for _, z := range m.layout().zones {
					if mo.Y == z.y && mo.X >= z.x0 && mo.X < z.x1 {
						return m.do(z.act)
					}
				}
			}
			return m, nil
		}
	}
	if m.judgeOpen {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			return m.updateJudgePicker(key)
		}
		if click, ok := msg.(tea.MouseClickMsg); ok {
			mo := click.Mouse()
			if mo.Button == tea.MouseLeft {
				for _, z := range m.layout().zones {
					if mo.Y == z.y && mo.X >= z.x0 && mo.X < z.x1 {
						return m.do(z.act)
					}
				}
			}
			return m, nil
		}
	}
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
			eligible := 0
			for _, a := range m.agents {
				if a.Status == "idle" || a.Status == "done" {
					eligible++
				}
			}
			for i, a := range m.agents {
				m.picked[a.Pane] = eligible <= 6 && (a.Status == "idle" || a.Status == "done")
				if a.Name == "claude" && m.judge < 0 {
					m.judge = i
				}
			}
			m.syncPeerDefault()
		}
		if m.judge < 0 || m.judge >= len(m.agents) {
			m.judge = 0
		}
		if m.cursor >= len(m.agents) {
			m.cursor = max(0, len(m.agents)-1)
		}
		m.keepSeatVisible()
		if m.run != nil && !m.oldRun {
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

	case seatSentMsg:
		if m.run != nil && m.run.ID == msg.run {
			m.run.sent(msg.seat, msg.err)
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
		mo := msg.Mouse()
		if m.page == "runs" {
			if mo.Button == tea.MouseWheelUp {
				m.runTop = max(0, m.runTop-2)
			} else {
				m.runTop = min(max(0, len(m.runs)-m.runsListHeight()), m.runTop+2)
			}
			m.runCursor = min(max(m.runCursor, m.runTop), min(max(0, len(m.runs)-1), m.runTop+m.runsListHeight()-1))
			return m, nil
		}
		if m.judgeOpen {
			if mo.Button == tea.MouseWheelUp {
				m.judgeCursor = max(0, m.judgeCursor-1)
			} else {
				m.judgeCursor = min(len(m.agents)-1, m.judgeCursor+1)
			}
			m.keepJudgeVisible()
			return m, nil
		}
		if m.page == "ask" && (m.focus == focusSeats || m.wheelOnSeats(mo.X, mo.Y)) {
			if mo.Button == tea.MouseWheelUp {
				m.seatTop = max(0, m.seatTop-2)
			} else {
				m.seatTop = min(max(0, len(m.agents)-m.seatListHeight()), m.seatTop+2)
			}
			if m.focus == focusSeats && len(m.agents) > 0 {
				m.cursor = min(max(m.cursor, m.seatTop), min(len(m.agents)-1, m.seatTop+m.seatListHeight()-1))
			}
			return m, nil
		}
		if m.page == "run" && m.run != nil {
			var cmd tea.Cmd
			m.answer, cmd = m.answer.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.phase != welcoming {
			if m.page == "runs" {
				return m.updateRuns(msg)
			}
			if m.page == "ask" && m.phase == watching {
				return m.updateAsking(msg)
			}
		}
		if m.phase == welcoming {
			return m.updateWelcome(msg)
		}
		if m.phase == asking {
			return m.updateAsking(msg)
		}
		return m.updateWatching(msg)
	}
	if m.page == "ask" && m.focus == focusQuestion {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

// do runs one named action: shared by keys and mouse clicks.
func (m model) do(act string) (tea.Model, tea.Cmd) {
	switch {
	case act == "nav-ask":
		m.page, m.settingsOpen = "ask", false
		m.focus = focusQuestion
		m.resize()
		return m, m.input.Focus()
	case act == "nav-runs":
		m.page, m.settingsOpen = "runs", false
		m.runs = listRuns(m.workspace)
		m.runCursor = min(max(0, m.runCursor), max(0, len(m.runs)-1))
		m.runTop = min(m.runTop, m.runCursor)
		m.input.Blur()
		return m, nil
	case act == "nav-settings":
		if m.page != "settings" {
			m.settingsReturn = m.page
		}
		m.page, m.settingsOpen = "settings", true
		m.settingsFocus, m.preferenceNote = 0, ""
		m.input.Blur()
		return m, nil
	case act == "nav-run" && m.run != nil:
		m.page, m.settingsOpen = "run", false
		m.refreshAnswer()
		return m, nil
	case strings.HasPrefix(act, "open-run:"):
		var i int
		fmt.Sscanf(act, "open-run:%d", &i)
		if i >= 0 && i < len(m.runs) {
			m.run, m.phase, m.page, m.seat, m.oldRun = m.runs[i], watching, "run", 0, true
			m.showReview = false
			m.sawVerdict = m.run.Judge != nil && m.run.Judge.State == Done
			m.refreshAnswer()
		}
		return m, nil
	case strings.HasPrefix(act, "welcome-") || strings.HasPrefix(act, "settings-"):
		m = m.preferenceAction(act)
		if act == "settings-close" && m.page == "ask" && m.focus == focusQuestion {
			return m, m.input.Focus()
		}
		return m, nil
	case act == "close":
		return m, tea.Quit
	case act == "help":
		m.help = true
		return m, nil
	case act == "settings":
		return m.do("nav-settings")
	case act == "ask":
		if strings.TrimSpace(m.input.Value()) == "" || m.selectedCount() == 0 {
			return m, nil
		}
		m.focus = focusAsk
		m.input.Blur()
		return m.ask()
	case act == "confirm-cancel":
		m.confirmAsk = false
		return m, nil
	case act == "seat-all", act == "seat-none":
		m.focus = focusSeats
		m.input.Blur()
		m.confirmAsk = false
		for _, a := range m.agents {
			m.picked[a.Pane] = act == "seat-all" && (a.Status == "idle" || a.Status == "done")
		}
		m.syncPeerDefault()
		return m, nil
	case act == "peer-toggle":
		m.focus = focusPeer
		m.input.Blur()
		m.peerReview = !m.peerReview
		m.peerChosen = true
		m.confirmAsk = false
		return m, nil
	case act == "input":
		m.focus = focusQuestion
		return m, m.input.Focus()
	case strings.HasPrefix(act, "toggle:"):
		var i int
		fmt.Sscanf(act, "toggle:%d", &i)
		if i >= 0 && i < len(m.agents) {
			m.focus = focusSeats
			m.input.Blur()
			p := m.agents[i].Pane
			m.picked[p] = !m.picked[p]
			m.syncPeerDefault()
			m.cursor = i
			m.confirmAsk = false
			m.keepSeatVisible()
		}
	case act == "judge":
		if len(m.agents) == 0 {
			m.flash = "no agents in this workspace yet"
			return m, nil
		}
		m.focus = focusJudge
		m.input.Blur()
		m.judgeOpen = true
		m.judgeCursor = max(0, m.judge)
		m.keepJudgeVisible()
		return m, nil
	case strings.HasPrefix(act, "judge-pick:"):
		var i int
		fmt.Sscanf(act, "judge-pick:%d", &i)
		if i >= 0 && i < len(m.agents) {
			m.judge = i
			m.judgeOpen = false
			m.confirmAsk = false
		}
		return m, nil
	case act == "judge-cancel":
		m.judgeOpen = false
		return m, nil
	case strings.HasPrefix(act, "seat:"):
		fmt.Sscanf(act, "seat:%d", &m.seat)
		m.showReview = false
	case act == "review", act == "answer":
		m.showReview = act == "review"
		m.answer.GotoTop()
	case act == "tab-prev", act == "tab-next":
		step := m.runListHeight()
		if act == "tab-prev" {
			step = -step
		}
		m.seat = min(len(m.run.Seats)-1, max(0, m.seat+step))
		m.showReview = false
	case act == "copy":
		if body := m.currentText(); body != "" && writeClipboard(body) == nil {
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
		if !m.oldRun && m.run != nil && m.run.Judge != nil && m.run.answered() >= 2 && m.run.Judge.Sent.IsZero() {
			m.run.startJudge()
			m.seat = len(m.run.Seats)
		}
	case act == "new":
		nm := newModel(m.workspace)
		nm.w, nm.h, nm.agents, nm.judge = m.w, m.h, m.agents, m.judge
		idle := 0
		for _, a := range nm.agents {
			if a.Status == "idle" || a.Status == "done" {
				idle++
			}
		}
		for _, a := range nm.agents {
			nm.picked[a.Pane] = idle <= 6 && (a.Status == "idle" || a.Status == "done")
		}
		nm.settings, nm.shortcut = m.settings, m.shortcut
		nm.peerReview, nm.peerChosen = m.peerReview, m.peerChosen
		nm.syncPeerDefault()
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
		if m.confirmAsk {
			m.confirmAsk = false
			return m, nil
		}
		return m, tea.Quit
	case "a", "n":
		if m.focus == focusSeats {
			if msg.String() == "a" {
				return m.do("seat-all")
			}
			return m.do("seat-none")
		}
	case "s":
		if m.focus != focusQuestion {
			return m.do("settings")
		}
	case "tab":
		return m.moveAskFocus(1)
	case "shift+tab":
		return m.moveAskFocus(-1)
	case "up", "k":
		if m.focus == focusSeats {
			m.cursor = max(0, m.cursor-1)
			m.keepSeatVisible()
			return m, nil
		}
		if m.focus != focusQuestion || msg.String() == "up" {
			return m.moveAskFocus(-1)
		}
	case "down", "j":
		if m.focus == focusSeats {
			m.cursor = min(max(0, len(m.agents)-1), m.cursor+1)
			m.keepSeatVisible()
			return m, nil
		}
		if m.focus != focusQuestion || msg.String() == "down" {
			return m.moveAskFocus(1)
		}
	case "enter":
		switch m.focus {
		case focusQuestion, focusAsk:
			return m.ask()
		case focusSeats:
			if len(m.agents) > 0 {
				return m.do(fmt.Sprintf("toggle:%d", m.cursor))
			}
		case focusJudge:
			return m.do("judge")
		case focusPeer:
			return m.do("peer-toggle")
		case focusClose:
			return m.do("close")
		}
		return m, nil
	case "space", "x":
		if m.focus == focusSeats && len(m.agents) > 0 {
			return m.do(fmt.Sprintf("toggle:%d", m.cursor))
		}
		if m.focus == focusPeer {
			return m.do("peer-toggle")
		}
	case "left", "right":
		if m.focus == focusSeats {
			step := 1
			if msg.String() == "left" {
				step = -1
			}
			m.cursor = min(max(0, len(m.agents)-1), max(0, m.cursor+step))
			m.keepSeatVisible()
			return m, nil
		}
		if m.focus == focusJudge {
			if len(m.agents) > 0 {
				step := 1
				if msg.String() == "left" {
					step = -1
				}
				m.judge = (m.judge + len(m.agents) + step) % len(m.agents)
				m.confirmAsk = false
			}
			return m, nil
		}
	case "J":
		if m.focus != focusQuestion {
			return m.do("judge")
		}
	}
	if m.focus != focusQuestion {
		return m, nil
	}
	m.confirmAsk = false
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) moveAskFocus(step int) (tea.Model, tea.Cmd) {
	m.focus = (m.focus + askFocus(int(focusCount)+step)) % focusCount
	if m.focus == focusQuestion {
		return m, m.input.Focus()
	}
	m.input.Blur()
	return m, nil
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
	if len(seats) > 8 && !m.confirmAsk {
		m.confirmAsk = true
		m.flash = ""
		return m, nil
	}
	m.confirmAsk = false
	r, err := newRun(q, seats, m.agents[m.judge])
	if err != nil {
		m.flash = err.Error()
		return m, nil
	}
	r.Workspace = m.workspace
	r.PeerReview = m.peerReview
	send := r.dispatch()
	m.run, m.seat, m.phase, m.page, m.flash, m.sawVerdict, m.oldRun = r, 0, watching, "run", "", false, false
	m.refreshAnswer()
	return m, send
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
	case "[", "pgup":
		m.seat = max(0, m.seat-m.runListHeight())
	case "]", "pgdown":
		m.seat = min(n-1, m.seat+m.runListHeight())
	case "v":
		m.seat = len(m.run.Seats)
	case "r":
		if m.reviewAvailable() {
			m.showReview = !m.showReview
			m.answer.GotoTop()
			m.refreshAnswer()
		}
		return m, nil
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
	m.showReview = false
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
	if m.onVerdict() {
		var parts []string
		if ranking := m.run.RankingText(s != nil && s.State == Done); ranking != "" {
			parts = append(parts, "PEER RANKING\n"+ranking)
		}
		if s != nil && s.State == Done {
			parts = append(parts, s.Answer, "Revealed: "+strings.Join(m.run.Reveal(), ", "))
		}
		return strings.Join(parts, "\n\n")
	}
	if m.showReview && m.reviewAvailable() {
		s = m.run.Reviews[m.seat]
	}
	if s == nil || s.State != Done {
		return ""
	}
	return s.Answer
}

func (m *model) refreshAnswer() {
	if m.run == nil {
		return
	}
	width, height := m.contentSize()
	_, rightWidth := runPaneWidths(width)
	m.answer.SetWidth(max(1, rightWidth-2))
	header := m.runHeader(rightWidth)
	actions := m.runActions(rightWidth)
	m.answer.SetHeight(max(1, height-len(header.lines)-len(actions.lines)-1))
	if m.onVerdict() {
		m.answer.SetContent(m.verdictBody(m.answer.Width()))
		return
	}
	s := m.run.Seats[m.seat]
	if m.showReview && m.reviewAvailable() {
		s = m.run.Reviews[m.seat]
	}
	var body string
	switch s.State {
	case Done:
		m.answer.SetContent(markdownBody(s.Answer, m.answer.Width()))
		return
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
	m.answer.SetContent(wrapBody(body, m.answer.Width()))
}

func (m model) verdictStatus(width int) string {
	r, j := m.run, m.run.Judge
	if j == nil {
		return dim.Render("No verdict is available for this run.")
	}
	switch {
	case j.State == Done:
		return markdownBody(j.Answer, width)
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

func (m model) View() tea.View {
	v := tea.NewView(strings.Join(m.layout().lines, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// headless asks without the UI: send, poll until the seats and the judge settle, print everything.
func headless(question, seatList, judgeName, ws, peer string) int {
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
		if (len(want) == 0 && (a.Status == "idle" || a.Status == "done")) || want[a.Name] || want[a.Pane] {
			picked = append(picked, a)
		}
		if a.Name == judgeName || a.Pane == judgeName {
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
	r.Workspace = ws
	r.PeerReview = peer == "on" || (peer == "auto" && len(picked) <= 6)
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
	if len(r.Ranking) > 0 {
		fmt.Printf("== peer ranking\n%s\n\n", r.RankingText(true))
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
	seats := flag.String("seats", "", "comma-separated agent names or pane ids to ask with --ask (default: all idle)")
	judge := flag.String("judge", "claude", "agent name that judges with --ask (default: claude, else the first seat)")
	peer := flag.String("peer-review", "auto", "with --ask: on, off, or auto (on for up to 6 seats)")
	flag.Parse()
	if *ask != "" {
		os.Exit(headless(*ask, *seats, *judge, *ws, *peer))
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
	m := launchModel(w)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "council:", err)
		os.Exit(1)
	}
}
