package main

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// The UI indirection keeps tests away from the user's Herdr configuration.
var bindShortcutFn = bindShortcut
var currentShortcutFn = currentShortcut

const reopenCommand = "herdr plugin action invoke tab --plugin herdr-council"

func launchModel(workspace string) model {
	m := newModel(workspace)
	m.settings = loadSettings()
	m.shortcut = currentShortcutFn()
	if !m.settings.Welcomed {
		m.phase = welcoming
		m.input.Blur()
		return m
	}
	if r := latestRun(30*time.Minute, m.workspace); r != nil {
		m.run, m.phase = r, watching
		m.sawVerdict = r.Judge != nil && r.Judge.State == Done
	}
	return m
}

func (m model) welcomeLayout() screen {
	var sc screen
	width := max(20, m.w)
	formWidth := width
	artX, showArt := 0, m.w >= 88 && m.h >= 28
	if showArt {
		artX = m.w - artWidth - 4
		formWidth = artX - 3
	}
	sc.add(seg(" " + bold.Render("WELCOME TO COUNCIL")))
	sc.add(seg(" " + dim.Render(shorten("One question. Independent answers. A blind verdict.", formWidth-1))))
	sc.blank()
	sc.add(seg(" " + accent.Render("01") + "  Ask one question"))
	sc.add(seg(" " + accent.Render("02") + "  Seats answer on their own"))
	sc.add(seg(" " + accent.Render("03") + "  Judge reads A/B/C; names reveal after verdict"))
	sc.blank()
	sc.add(seg(" " + bold.Render("GETTING BACK TO COUNCIL")))
	sc.add(seg(" " + dim.Render("Council tab in Herdr's tab bar")))
	shortcutWay := "Optional shortcut: add one below"
	if m.shortcut != "" {
		shortcutWay = "Shortcut: " + m.shortcut
	}
	sc.add(seg(" " + dim.Render(shorten(shortcutWay, formWidth-1))))
	sc.add(seg(" " + dim.Render(shorten(reopenCommand, formWidth-1))))
	sc.blank()
	auto := "[ ]"
	if m.settings.AutoTab {
		auto = "[x]"
	}
	sc.add(act(askFocusMark(m.welcomeFocus == 0)+accent.Render(auto)+" "+text.Render("Keep a Council tab in every workspace"), "welcome-auto"))
	sc.add(seg("    " + dim.Render(shorten("Closed tabs return on the next Herdr start.", formWidth-4))))
	sc.add(welcomeShortcutButton(m, "welcome-shortcut")...)
	if m.preferenceNote != "" {
		sc.add(seg("    " + warn.Render(shorten(m.preferenceNote, formWidth-4))))
	}
	sc.blank()
	start := button
	if m.welcomeFocus == 2 {
		start = primary
	}
	sc.add(act(askFocusMark(m.welcomeFocus == 2)+start.Render("Start"), "welcome-start"))
	if m.h >= 24 {
		for len(sc.lines) < m.h-1 {
			sc.blank()
		}
	}
	for _, row := range keyLegend([]legendItem{{"tab/S-tab", "focus", ""}, {"enter/space", "use", ""}, {"?", "help", "help"}}, width) {
		sc.add(row...)
	}
	if showArt {
		placeCouncilArt(&sc, artX)
	}
	return sc
}

func welcomeShortcutButton(m model, action string) [][2]string {
	mark := askFocusMark(m.welcomeFocus == 1)
	if m.shortcut != "" {
		return [][2]string{seg(mark + dim.Render("[ Add shortcut ]") + "  " + good.Render("Council is on "+m.shortcut))}
	}
	style := button
	if m.welcomeFocus == 1 {
		style = primary
	}
	return [][2]string{act(mark+style.Render("Add shortcut"), action)}
}

func (m model) moveWelcomeFocus(step int) model {
	for {
		m.welcomeFocus = (m.welcomeFocus + 3 + step) % 3
		if m.welcomeFocus != 1 || m.shortcut == "" {
			return m
		}
	}
}

func (m model) updateWelcome(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		return m, tea.Quit
	case "tab", "down":
		return m.moveWelcomeFocus(1), nil
	case "shift+tab", "up":
		return m.moveWelcomeFocus(-1), nil
	case "enter", "space":
		switch m.welcomeFocus {
		case 0:
			return m.do("welcome-auto")
		case 1:
			return m.do("welcome-shortcut")
		default:
			return m.do("welcome-start")
		}
	}
	return m, nil
}

func (m model) moveSettingsFocus(step int) model {
	for {
		m.settingsFocus = (m.settingsFocus + 3 + step) % 3
		if m.settingsFocus != 1 || m.shortcut == "" {
			return m
		}
	}
}

func (m model) updateSettings(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "s":
		return m.do("settings-close")
	case "tab", "down":
		return m.moveSettingsFocus(1), nil
	case "shift+tab", "up":
		return m.moveSettingsFocus(-1), nil
	case "enter", "space":
		switch m.settingsFocus {
		case 0:
			return m.do("settings-auto")
		case 1:
			return m.do("settings-shortcut")
		default:
			return m.do("settings-close")
		}
	}
	return m, nil
}

func (m model) preferenceAction(action string) model {
	switch action {
	case "welcome-auto", "settings-auto":
		if action == "welcome-auto" {
			m.welcomeFocus = 0
		} else {
			m.settingsFocus = 0
		}
		next := m.settings
		next.AutoTab = !next.AutoTab
		if err := saveSettings(next); err != nil {
			m.preferenceNote = err.Error()
		} else {
			m.settings = next
			m.preferenceNote = "Saved; tab choice applies on the next Herdr start."
		}
	case "welcome-shortcut", "settings-shortcut":
		if m.shortcut != "" {
			return m
		}
		if action == "welcome-shortcut" {
			m.welcomeFocus = 1
		} else {
			m.settingsFocus = 1
		}
		key, _, err := bindShortcutFn("")
		if err != nil {
			m.preferenceNote = err.Error()
		} else {
			m.shortcut = key
			m.preferenceNote = "Council is on " + key
			if action == "welcome-shortcut" {
				m.welcomeFocus = 2
			} else {
				m.settingsFocus = 2
			}
		}
	case "welcome-start":
		m.welcomeFocus = 2
		next := m.settings
		next.Welcomed = true
		if err := saveSettings(next); err != nil {
			m.preferenceNote = err.Error()
		} else {
			m.settings = next
			m.phase = asking
			m.preferenceNote = ""
			m.input.Focus()
			m.resize()
		}
	case "settings-close":
		m.settingsOpen = false
		m.preferenceNote = ""
	}
	return m
}

func (m model) settingsOverlay(base screen) screen {
	width, height := max(20, m.w), max(10, m.h)
	panelWidth := min(70, width-4)
	var panel screen
	border := dim.Render("+" + strings.Repeat("-", panelWidth-2) + "+")
	panel.add(seg(border))
	panel.add(seg("| " + bold.Render("⚙ SETTINGS") + strings.Repeat(" ", panelWidth-4-lipgloss.Width("⚙ SETTINGS")) + " |"))
	panel.add(seg("| " + paddedCell(dim.Render("Choose how Council stays within reach."), panelWidth-4) + " |"))
	panel.add(seg(border))
	auto := "[ ]"
	if m.settings.AutoTab {
		auto = "[x]"
	}
	autoLine := askFocusMark(m.settingsFocus == 0) + accent.Render(auto) + " Keep a Council tab in every workspace"
	panel.add(seg("| "), act(paddedCell(autoLine, panelWidth-4), "settings-auto"), seg(" |"))
	panel.add(seg("| " + paddedCell(dim.Render("  A closed tab returns at the next Herdr start."), panelWidth-4) + " |"))
	shortcutLine := ""
	if m.shortcut != "" {
		shortcutLine = askFocusMark(m.settingsFocus == 1) + dim.Render("[ Add shortcut ]") + "  " + good.Render("Council is on "+m.shortcut)
		panel.add(seg("| " + paddedCell(shortcutLine, panelWidth-4) + " |"))
	} else {
		style := button
		if m.settingsFocus == 1 {
			style = primary
		}
		btn := askFocusMark(m.settingsFocus == 1) + style.Render("Add shortcut")
		panel.add(seg("| "), act(btn, "settings-shortcut"), seg(strings.Repeat(" ", panelWidth-4-lipgloss.Width(btn))+" |"))
	}
	if m.preferenceNote != "" {
		panel.add(seg("| " + paddedCell(warn.Render(shorten(m.preferenceNote, panelWidth-4)), panelWidth-4) + " |"))
	}
	panel.add(seg(border))
	panel.add(seg("| " + paddedCell(bold.Render("GETTING BACK TO COUNCIL"), panelWidth-4) + " |"))
	panel.add(seg("| " + paddedCell(dim.Render("Council tab  ·  "+shortcutWay(m.shortcut)), panelWidth-4) + " |"))
	if lipgloss.Width(reopenCommand) <= panelWidth-4 {
		panel.add(seg("| " + paddedCell(dim.Render(reopenCommand), panelWidth-4) + " |"))
	} else {
		panel.add(seg("| " + paddedCell(dim.Render("herdr plugin action invoke tab"), panelWidth-4) + " |"))
		panel.add(seg("| " + paddedCell(dim.Render("  --plugin herdr-council"), panelWidth-4) + " |"))
	}
	panel.add(seg(border))
	closeStyle := button
	if m.settingsFocus == 2 {
		closeStyle = primary
	}
	closeLabel := askFocusMark(m.settingsFocus == 2) + closeStyle.Render("Close")
	panel.add(seg("| "), act(closeLabel, "settings-close"), seg(strings.Repeat(" ", panelWidth-4-lipgloss.Width(closeLabel))+" |"))
	panel.add(seg("| " + paddedCell(dim.Render("tab/S-tab focus  ·  enter/space use  ·  esc/s close"), panelWidth-4) + " |"))
	panel.add(seg(border))
	return placeOverlay(base, panel, width, height)
}

func shortcutWay(key string) string {
	if key == "" {
		return "optional shortcut"
	}
	return key
}

func placeOverlay(base, panel screen, width, height int) screen {
	x, y := max(0, (width-lipgloss.Width(panel.lines[0]))/2), max(0, (height-len(panel.lines))/2)
	base.zones = nil
	for len(base.lines) < height {
		base.lines = append(base.lines, "")
	}
	for i, line := range panel.lines {
		if y+i >= len(base.lines) {
			break
		}
		base.lines[y+i] = strings.Repeat(" ", x) + line
	}
	for _, z := range panel.zones {
		base.zones = append(base.zones, zone{z.y + y, z.x0 + x, z.x1 + x, z.act})
	}
	return base
}
