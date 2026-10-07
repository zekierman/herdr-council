package main

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
		m.run, m.phase, m.page = r, watching, "run"
		m.sawVerdict = r.Judge != nil && r.Judge.State == Done
	}
	return m
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
		m.page = m.settingsReturn
		if m.page == "" {
			m.page = "ask"
		}
		m.preferenceNote = ""
	}
	return m
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
		base.lines[y+i] = fit(ansi.Cut(base.lines[y+i], 0, x), x) + line + ansi.Cut(base.lines[y+i], x+lipgloss.Width(line), width)
	}
	for _, z := range panel.zones {
		if z.y+y < height {
			base.zones = append(base.zones, zone{z.y + y, z.x0 + x, z.x1 + x, z.act})
		}
	}
	return base
}
