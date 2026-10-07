package main

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) > width {
		s = ansi.Truncate(s, width, "")
	}
	return s + strings.Repeat(" ", max(0, width-lipgloss.Width(s)))
}

func (m model) contentSize() (int, int) {
	if m.w < 72 {
		return max(16, m.w-4), max(8, m.h-3)
	}
	return max(16, m.w-18), max(8, m.h-2)
}

func runPaneWidths(width int) (int, int) {
	left := min(25, max(20, width/3))
	return left, max(1, width-left-1)
}

func (m model) switchPage(step int) (tea.Model, tea.Cmd) {
	pages := []string{"ask", "runs", "settings"}
	if m.run != nil {
		pages = append(pages, "run")
	}
	i := 0
	for j, page := range pages {
		if page == m.page {
			i = j
			break
		}
	}
	return m.do("nav-" + pages[(i+len(pages)+step)%len(pages)])
}

// compose pads both columns and translates/clips zones by the same cell offsets as the text.
func compose(left, right screen, leftWidth, rightWidth, height int) screen {
	var out screen
	for y := 0; y < height; y++ {
		l, r := "", ""
		if y < len(left.lines) {
			l = left.lines[y]
		}
		if y < len(right.lines) {
			r = right.lines[y]
		}
		out.lines = append(out.lines, fit(l, leftWidth)+colour(faintHex).Render("│")+fit(r, rightWidth))
	}
	for _, z := range left.zones {
		if z.y < height && z.x0 < leftWidth {
			z.x1 = min(z.x1, leftWidth)
			out.zones = append(out.zones, z)
		}
	}
	for _, z := range right.zones {
		if z.y < height && z.x0 < rightWidth {
			z.x0 += leftWidth + 1
			z.x1 = leftWidth + 1 + min(z.x1, rightWidth)
			out.zones = append(out.zones, z)
		}
	}
	return out
}

func roundedPanel(body screen, width int, title string) screen {
	var out screen
	out.add(seg(colour(faintHex).Render("╭─ ")), seg(bold.Render(title)), seg(colour(faintHex).Render(" "+strings.Repeat("─", max(0, width-lipgloss.Width(title)-5))+"╮")))
	for _, line := range body.lines {
		out.add(seg(colour(faintHex).Render("│ ") + fit(line, width-4) + colour(faintHex).Render(" │")))
	}
	for _, z := range body.zones {
		if z.x0 < width-4 {
			out.zones = append(out.zones, zone{z.y + 1, z.x0 + 2, min(z.x1, width-4) + 2, z.act})
		}
	}
	out.add(seg(colour(faintHex).Render("╰" + strings.Repeat("─", width-2) + "╯")))
	return out
}

// The interior starts two cells from the left and one row below the top border.
func (m model) renderFrame(inner screen, title, context string) screen {
	w, h := max(20, m.w), max(10, m.h)
	inside := w - 4
	var out screen
	if w < 72 {
		context = strings.ReplaceAll(context, "Peer review", "Review")
	}
	context = shorten(context, max(0, w-lipgloss.Width(title)-9))
	gap := max(0, w-lipgloss.Width(title)-lipgloss.Width(context)-8)
	out.add(seg(colour(faintHex).Render("╭─ ")), seg(bold.Render(title)), seg(colour(faintHex).Render(" "+strings.Repeat("─", gap)+" ")), seg(dim.Render(context)), seg(colour(faintHex).Render(" ─╮")))
	for y := 0; y < h-2; y++ {
		line := ""
		if y < len(inner.lines) {
			line = inner.lines[y]
		}
		out.add(seg(colour(faintHex).Render("│ ") + fit(line, inside) + colour(faintHex).Render(" │")))
	}
	for _, z := range inner.zones {
		if z.y < h-2 && z.x0 < inside {
			out.zones = append(out.zones, zone{z.y + 1, z.x0 + 2, min(z.x1, inside) + 2, z.act})
		}
	}
	hints := "? help · q quit"
	if m.page == "run" {
		hints = "c copy · r review · n new · ? help"
	}
	if m.page == "ask" {
		hints = "tab focus · ? help · esc quit"
	}
	if m.page == "runs" {
		hints = "↑↓ select · enter open · ? help"
	}
	if m.page == "settings" {
		hints = "tab focus · enter select · esc back"
	}
	if m.phase == welcoming {
		hints = "tab focus · enter select · ? help"
	}
	hintStyle := dim
	if m.page == "ask" && m.focus == focusClose {
		hintStyle = accent
	}
	out.add(seg(colour(faintHex).Render("╰"+strings.Repeat("─", max(0, w-5-lipgloss.Width(hints)))+" ")), seg(hintStyle.Render(hints)), seg(colour(faintHex).Render(" ─╯")))
	return out
}

func (m model) layout() screen {
	if m.help {
		base := m
		base.help = false
		return helpOverlay(base.layout(), m.w, m.h)
	}
	if m.judgeOpen {
		base := m
		base.judgeOpen = false
		return m.judgePickerOverlay(base.layout())
	}
	if m.phase == welcoming {
		return m.framedWelcome()
	}
	page := m.page
	if page == "" {
		if m.phase == watching {
			page = "run"
		} else {
			page = "ask"
		}
	}
	if m.settingsOpen {
		page = "settings"
	}
	innerWidth, innerHeight := max(16, m.w-4), max(8, m.h-2)
	var inner screen
	if m.w < 72 {
		inner.add(m.navTabs(page, innerWidth)...)
		var body screen
		m.pageBody(&body, page, innerWidth, innerHeight-1)
		for _, line := range body.lines {
			inner.lines = append(inner.lines, line)
		}
		for _, z := range body.zones {
			z.y++
			inner.zones = append(inner.zones, z)
		}
	} else {
		var nav, body screen
		m.navColumn(&nav, page, 13, innerHeight)
		m.pageBody(&body, page, innerWidth-14, innerHeight)
		inner = compose(nav, body, 13, innerWidth-14, innerHeight)
	}
	ctx := fmt.Sprintf("● %d agents", len(m.agents))
	if page == "run" && m.run != nil {
		ctx = m.stageLine()
	}
	return m.renderFrame(inner, "Council", ctx)
}

func (m model) navItems() [][2]string {
	items := [][2]string{{"Ask", "nav-ask"}, {"Runs", "nav-runs"}, {"Settings", "settings"}}
	if m.run != nil {
		items = append(items, [2]string{"Current run", "nav-run"})
	}
	return items
}

func (m model) navColumn(sc *screen, page string, width, height int) {
	for _, item := range m.navItems() {
		name := item[0]
		active := strings.EqualFold(name, page) || (name == "Current run" && page == "run")
		label := "  " + name
		if active {
			label = accent.Render("▸ " + name)
		} else {
			label = dim.Render(label)
		}
		sc.add(act(fit(label, width), item[1]))
	}
}

func (m model) navTabs(page string, width int) [][2]string {
	row := [][2]string{}
	for _, item := range m.navItems() {
		name := item[0]
		if name == "Current run" {
			name = "Run"
		}
		active := strings.EqualFold(item[0], page) || (name == "Run" && page == "run")
		label := " " + name + " "
		if active {
			label = accent.Render("▸" + label)
		} else {
			label = dim.Render(label)
		}
		row = append(row, act(label, item[1]), seg(" "))
	}
	return row
}

func (m model) pageBody(sc *screen, page string, width, height int) {
	switch page {
	case "runs":
		m.runsPage(sc, width, height)
	case "settings":
		m.settingsPage(sc, width, height)
	case "run":
		if m.run != nil {
			m.runPage(sc, width, height)
		} else {
			m.askPage(sc, width, height)
		}
	default:
		m.askPage(sc, width, height)
	}
}

func (m model) askPage(sc *screen, width, height int) {
	pad := 2
	fieldWidth := max(12, width-2*pad)
	sc.add(seg("  " + bold.Render("Question")))
	border := faintHex
	if m.focus == focusQuestion {
		border = accentHex
	}
	sc.add(seg("  " + colour(border).Render("╭"+strings.Repeat("─", fieldWidth-2)+"╮")))
	inputLines := strings.Split(m.input.View(), "\n")
	for i := 0; i < 3; i++ {
		line := ""
		if i < len(inputLines) {
			line = inputLines[i]
		}
		sc.add(seg("  " + colour(border).Render("│") + fit(" "+line, fieldWidth-2) + colour(border).Render("│")))
	}
	sc.add(seg("  " + colour(border).Render("╰"+strings.Repeat("─", fieldWidth-2)+"╯")))
	for y := 2; y < 5; y++ {
		sc.zones = append(sc.zones, zone{y, 2, 2 + fieldWidth, "input"})
	}
	sc.blank()
	count := fmt.Sprintf("%d of %d", m.selectedCount(), len(m.agents))
	sc.add(seg("  " + bold.Render("Seats") + strings.Repeat(" ", max(1, width-4-lipgloss.Width("Seats")-len(count))) + dim.Render(count)))
	sc.add(seg("  "), act(dim.Render("All"), "seat-all"), seg(dim.Render("  ·  ")), act(dim.Render("None"), "seat-none"))
	if m.agentErr != "" {
		sc.add(seg("  " + bad.Render(shorten(m.agentErr, width-4))))
	}
	if len(m.agents) == 0 && m.agentErr == "" {
		sc.add(seg("  " + dim.Render("No agents in this workspace.")))
	}
	top, bottom := m.visibleSeats()
	if top > 0 {
		sc.add(seg("  " + dim.Render(fmt.Sprintf("↑ %d more", top))))
	}
	if len(m.agents) <= 3 && width >= 40 {
		row := [][2]string{seg("  ")}
		for i := top; i < bottom; i++ {
			a := m.agents[i]
			mark, name := "○", dim.Render(a.Name)
			if m.picked[a.Pane] {
				mark, name = "●", agentName(a.Name)
			}
			dotStyle := dim
			if m.picked[a.Pane] {
				dotStyle = colour(agentColour(a.Name))
			}
			label := dotStyle.Render(mark) + " " + name
			if m.focus == focusSeats && m.cursor == i {
				label = accent.Render("▸") + label
			}
			row = append(row, act(label, fmt.Sprintf("toggle:%d", i)), seg("  "))
		}
		sc.add(row...)
	} else {
		for i := top; i < bottom; i++ {
			a := m.agents[i]
			mark := "○"
			style := dim
			if m.picked[a.Pane] {
				mark = "●"
				style = colour(agentColour(a.Name))
			}
			name := agentName(a.Name)
			if !m.picked[a.Pane] {
				name = dim.Render(a.Name)
			}
			label := style.Render(mark) + " " + name
			duplicates := 0
			for _, other := range m.agents {
				if other.Name == a.Name {
					duplicates++
				}
			}
			if duplicates > 1 {
				hint := a.Pane
				if hint == "" {
					hint = a.Title
				}
				label += " " + dim.Render(shorten(hint, max(1, width-lipgloss.Width(label)-7)))
			}
			if m.focus == focusSeats && m.cursor == i {
				label = accent.Render("▸ ") + label
			} else {
				label = "  " + label
			}
			if width >= 40 && bottom-top <= 4 {
				label = "  " + label
			}
			sc.add(seg("  "), act(fit(label, width-4), fmt.Sprintf("toggle:%d", i)))
		}
	}
	if bottom < len(m.agents) {
		sc.add(seg("  " + dim.Render(fmt.Sprintf("↓ %d more", len(m.agents)-bottom))))
	}
	sc.blank()
	judge := "none"
	if m.judge >= 0 && m.judge < len(m.agents) {
		judge = m.agents[m.judge].Name
	}
	judgeStyle := text
	if m.focus == focusJudge {
		judgeStyle = accent
	}
	sc.add(seg("  "+bold.Render("Judge")+"  "), act(judgeStyle.Render("[ "+shorten(judge, 12)+" ▾ ]"), "judge"))
	peer := "[ OFF ]"
	peerStyle := dim
	if m.peerReview {
		peer = "[ ON ]"
		peerStyle = accent
	}
	peerTitle := bold.Render("Peer review")
	if m.focus == focusPeer {
		peerTitle = accent.Render("▸ Peer review")
	}
	sc.add(seg("  "+peerTitle+"  "), act(peerStyle.Render(peer), "peer-toggle"))
	sc.blank()
	label := "Ask council"
	if m.confirmAsk {
		label = fmt.Sprintf("Ask %d agents? Confirm", m.selectedCount())
	}
	btn := primary
	buttonBorder := accentHex
	if strings.TrimSpace(m.input.Value()) == "" || m.selectedCount() == 0 {
		btn = dim.Background(lipgloss.Color(faintHex))
		buttonBorder = faintHex
	}
	roundedPrimaryButton(sc, label, "ask", max(2, (width-lipgloss.Width(label)-6)/2), m.focus == focusAsk, btn, buttonBorder)
	if m.confirmAsk {
		sc.add(seg("  " + warn.Render("Enter or click to confirm · Esc cancel")))
	}
	if m.flash != "" {
		sc.add(seg("  " + warn.Render(shorten(m.flash, width-4))))
	}
}

func (m model) runListHeight() int { return max(1, min(len(m.run.Seats), m.h-13)) }

func (m model) runHeader(width int) screen {
	var sc screen
	heading := "VERDICT"
	if !m.onVerdict() {
		heading = "ANSWER · " + m.run.Seats[m.seat].Agent.Name
		if m.showReview {
			heading = "REVIEW · " + m.run.Seats[m.seat].Agent.Name
		}
	}
	sc.add(seg(" " + bold.Render(shorten(heading, width-2))))
	sc.add(seg(" " + dim.Render(shorten(oneLine(m.run.Question), width-2))))
	if m.reviewAvailable() {
		a, r := dim, dim
		if m.showReview {
			r = primary
		} else {
			a = primary
		}
		sc.add(seg(" "), act(a.Padding(0, 1).Render("Answer"), "answer"), seg(dim.Render(" │ ")), act(r.Padding(0, 1).Render("Review"), "review"))
	}
	sc.blank()
	return sc
}

func (m model) runActions(width int) screen {
	var sc screen
	items := [][2]string{act(button.Render("Copy"), "copy"), act(button.Render("Go to agent"), "goto"), act(button.Render("New question"), "new")}
	if !m.oldRun && m.run.answered() >= 2 && m.run.Judge != nil && m.run.Judge.Sent.IsZero() {
		items = append(items, act(primary.Render("Judge now"), "judge-now"))
	}
	row := [][2]string{seg(" ")}
	used := 1
	for _, item := range items {
		itemWidth := lipgloss.Width(item[0])
		if used > 1 && used+1+itemWidth > width {
			sc.add(row...)
			row = [][2]string{seg(" ")}
			used = 1
		}
		if used > 1 {
			row = append(row, seg(" "))
			used++
		}
		row = append(row, item)
		used += itemWidth
	}
	sc.add(row...)
	if m.flash != "" {
		sc.add(seg(" " + warn.Render(shorten(m.flash, width-2))))
	}
	return sc
}

func seatStateLabel(s *Seat) string {
	if s == nil {
		return "waiting"
	}
	if s.Sent.IsZero() && s.State != Done && s.State != Failed && s.State != Silent {
		return "waiting"
	}
	switch s.State {
	case Done:
		return "done"
	case Working:
		return "working"
	case Sending:
		return "sending"
	case Blocked:
		return "waiting"
	case Silent:
		return "silent"
	case Late:
		return "late"
	default:
		return "failed"
	}
}

func (m model) runPage(sc *screen, width, height int) {
	listWidth, rightWidth := runPaneWidths(width)
	var left, right screen
	left.add(seg(" " + dim.Render("SEATS")))
	visible := m.runListHeight()
	top := min(max(0, m.seat-visible/2), max(0, len(m.run.Seats)-visible))
	if top > 0 {
		left.add(act(fit(dim.Render(fmt.Sprintf(" ↑ %d more", top)), listWidth), "tab-prev"))
	}
	now := time.Now()
	for i := top; i < min(len(m.run.Seats), top+visible); i++ {
		s := m.run.Seats[i]
		state := seatStateLabel(s)
		statusSeat := s
		if i < len(m.run.Reviews) && m.run.Reviews[i] != nil && !m.run.Reviews[i].State.settled() {
			statusSeat = m.run.Reviews[i]
			state = "review"
		}
		elapsed := ""
		if !statusSeat.Sent.IsZero() {
			elapsed = fmt.Sprintf(" %ds", int(statusSeat.Elapsed(now).Seconds()))
		}
		if listWidth < 25 {
			switch state {
			case "working":
				state = "work"
			case "waiting":
				state = "wait"
			case "sending":
				state = "send"
			case "failed":
				state = "fail"
			}
		}
		status := state + elapsed
		pre := " "
		if i == m.seat {
			pre = accent.Render("▸")
		}
		nameWidth := max(1, listWidth-lipgloss.Width(status)-4)
		label := pre + stateMark(statusSeat, m.frame) + " " + shorten(agentName(s.Agent.Name), nameWidth) + " " + dim.Render(status)
		left.add(act(fit(label, listWidth), fmt.Sprintf("seat:%d", i)))
	}
	if top+visible < len(m.run.Seats) {
		left.add(act(fit(dim.Render(fmt.Sprintf(" ↓ %d more", len(m.run.Seats)-top-visible)), listWidth), "tab-next"))
	}
	left.add(seg(colour(faintHex).Render(strings.Repeat("─", listWidth))))
	starStyle := warn
	if m.run.Judge != nil && m.run.Judge.State == Done {
		starStyle = good
	} else if m.run.Judge != nil && m.run.Judge.State == Failed {
		starStyle = bad
	} else if m.run.Judge != nil && !m.run.Judge.Sent.IsZero() && (m.run.Judge.State == Working || m.run.Judge.State == Sending) {
		starStyle = accent
	} else if m.run.Judge == nil || (m.run.Judge.State == Silent) {
		starStyle = dim
	}
	verdict := " " + starStyle.Render("◆") + " Verdict " + dim.Render(seatStateLabel(m.run.Judge))
	if m.onVerdict() {
		verdict = accent.Render("▸") + verdict
	}
	left.add(act(fit(verdict, listWidth), fmt.Sprintf("seat:%d", len(m.run.Seats))))
	right = m.runHeader(rightWidth)
	actions := m.runActions(rightWidth)
	vp := m.answer
	vp.SetWidth(max(1, rightWidth-2))
	vp.SetHeight(max(1, height-len(right.lines)-len(actions.lines)-1))
	for _, line := range strings.Split(vp.View(), "\n") {
		right.add(seg(" " + fit(line, rightWidth-2)))
	}
	right.add(seg(colour(faintHex).Render(strings.Repeat("─", max(1, rightWidth)))))
	for _, z := range actions.zones {
		z.y += len(right.lines)
		right.zones = append(right.zones, z)
	}
	right.lines = append(right.lines, actions.lines...)
	*sc = compose(left, right, listWidth, rightWidth, height)
}

func (m model) runsPage(sc *screen, width, height int) {
	sc.add(seg("  " + bold.Render("Runs")))
	sc.add(seg("  " + dim.Render("Questions in this workspace · newest first")))
	sc.blank()
	if len(m.runs) == 0 {
		sc.add(seg("  " + dim.Render("No questions yet in this workspace.")))
		return
	}
	visible := m.runsListHeight()
	top := min(max(0, m.runTop), max(0, len(m.runs)-visible))
	if m.runCursor < top {
		top = m.runCursor
	}
	if m.runCursor >= top+visible {
		top = m.runCursor - visible + 1
	}
	for i := top; i < min(len(m.runs), top+visible); i++ {
		r := m.runs[i]
		when := "--:--"
		if len(r.Seats) > 0 && !r.Seats[0].Sent.IsZero() {
			when = r.Seats[0].Sent.Format("15:04")
		}
		state := stateMark(r.Judge, m.frame)
		if r.Judge == nil || (r.seatsSettled() && r.answered() < 2 && r.Judge.Sent.IsZero()) {
			state = dim.Render("○")
		}
		judge := ""
		if r.Judge != nil {
			judge = r.Judge.Agent.Name
		}
		suffix := fmt.Sprintf("  %d seats · %s", len(r.Seats), judge)
		question := shorten(oneLine(r.Question), max(8, width-11-lipgloss.Width(suffix)))
		label := "  " + state + " " + when + "  " + question + suffix
		if i == m.runCursor {
			label = accent.Render("▸") + label[1:]
		}
		sc.add(act(fit(label, width), fmt.Sprintf("open-run:%d", i)))
	}
}

func (m model) updateRuns(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up", "k":
		m.runCursor = max(0, m.runCursor-1)
	case "down", "j":
		m.runCursor = min(max(0, len(m.runs)-1), m.runCursor+1)
	case "enter":
		return m.do(fmt.Sprintf("open-run:%d", m.runCursor))
	case "esc", "q":
		return m.do("nav-ask")
	case "s":
		return m.do("nav-settings")
	}
	return m, nil
}

func (m model) runsListHeight() int { _, height := m.contentSize(); return max(1, height-3) }

func (m model) settingsPage(sc *screen, width, height int) {
	sc.add(seg("  " + bold.Render("Settings")))
	sc.add(seg("  " + dim.Render("Choose how Council stays within reach.")))
	sc.blank()
	auto := "[ OFF ]"
	if m.settings.AutoTab {
		auto = "[ ON ]"
	}
	autoStyle := dim
	if m.settings.AutoTab {
		autoStyle = accent
	}
	sc.add(act(askFocusMark(m.settingsFocus == 0)+bold.Render("Council tab in every workspace")+"  "+autoStyle.Render(auto), "settings-auto"))
	sc.add(seg("  " + dim.Render("Closed tabs return on the next Herdr start.")))
	sc.blank()
	if m.shortcut == "" {
		style := button
		if m.settingsFocus == 1 {
			style = primary
		}
		sc.add(seg(askFocusMark(m.settingsFocus == 1)), act(style.Render("Add shortcut"), "settings-shortcut"))
	} else {
		sc.add(seg("  " + good.Render("Council is on "+m.shortcut)))
	}
	if m.preferenceNote != "" {
		sc.add(seg("  " + warn.Render(shorten(m.preferenceNote, width-4))))
	}
	sc.blank()
	sc.add(seg("  " + bold.Render("Getting back to Council")))
	sc.add(seg("  " + dim.Render("Council tab · "+shortcutWay(m.shortcut))))
	sc.add(seg("  " + dim.Render(shorten(reopenCommand, width-4))))
	sc.blank()
	backStyle := button
	if m.settingsFocus == 2 {
		backStyle = primary
	}
	backLabel := "Back to Ask"
	if m.settingsReturn == "run" {
		backLabel = "Back to run"
	} else if m.settingsReturn == "runs" {
		backLabel = "Back to Runs"
	}
	sc.add(seg(askFocusMark(m.settingsFocus == 2)), act(backStyle.Render(backLabel), "settings-close"))
}

func (m model) framedWelcome() screen {
	width := max(16, m.w-4)
	formWidth := width
	showArt := m.w >= 88 && m.h >= 28
	artX := width - artWidth - 3
	if showArt {
		formWidth = artX - 2
	}
	var sc screen
	sc.add(seg("  " + bold.Render("WELCOME TO COUNCIL")))
	sc.add(seg("  " + dim.Render(shorten("One question. Independent answers. A blind verdict.", formWidth-2))))
	sc.blank()
	sc.add(seg("  " + accent.Render("Ask a question")))
	sc.add(seg("  " + dim.Render("Seats answer independently.")))
	sc.add(seg("  " + dim.Render("The judge reads anonymous views.")))
	sc.blank()
	sc.add(seg("  " + bold.Render("GETTING BACK TO COUNCIL")))
	sc.add(seg("  " + dim.Render("Council tab in Herdr's tab bar")))
	sc.add(seg("  " + dim.Render(shorten(reopenCommand, formWidth-2))))
	sc.blank()
	auto := "[ OFF ]"
	if m.settings.AutoTab {
		auto = "[ ON ]"
	}
	sc.add(seg(askFocusMark(m.welcomeFocus == 0)), act(accent.Render(auto)+" Keep a Council tab in every workspace", "welcome-auto"))
	sc.add(seg("  " + dim.Render("Closed tabs return on the next Herdr start.")))
	sc.add(append([][2]string{seg("  ")}, welcomeShortcutButton(m, "welcome-shortcut")...)...)
	if m.preferenceNote != "" {
		sc.add(seg("  " + warn.Render(shorten(m.preferenceNote, formWidth-2))))
	}
	sc.blank()
	roundedPrimaryButton(&sc, "Start", "welcome-start", 2, m.welcomeFocus == 2, primary, accentHex)
	if showArt {
		placeCouncilArt(&sc, artX)
	}
	return m.renderFrame(sc, "Council", "Welcome")
}

// Only the label carries a background. Borders, focus mark and frame padding
// are separate spans, so a primary button cannot expand into a coloured row.
func roundedPrimaryButton(sc *screen, label, action string, x int, focused bool, style lipgloss.Style, border string) {
	width := lipgloss.Width(label) + 6
	prefix := strings.Repeat(" ", x)
	sc.add(seg(prefix + colour(border).Render("╭"+strings.Repeat("─", width-2)+"╮")))
	if focused {
		prefix = strings.Repeat(" ", max(0, x-2)) + accent.Render("▸ ")
	}
	sc.add(seg(prefix+colour(border).Render("│")), act(style.Padding(0).Render("  "+label+"  ")+"\x1b[49m", action), seg(colour(border).Render("│")))
	sc.add(seg(strings.Repeat(" ", x) + colour(border).Render("╰"+strings.Repeat("─", width-2)+"╯")))
}
