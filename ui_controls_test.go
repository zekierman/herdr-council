package main

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestT23WelcomeSnippet(t *testing.T) {
	m := askingModel(140, 40)
	m.phase = welcoming
	for _, focus := range []int{0, 2} {
		m.welcomeFocus = focus
		sc := m.layout()
		for _, z := range sc.zones {
			if z.act == "welcome-start" {
				if z.x1-z.x0 != 9 {
					t.Fatal("Start expanded beyond its label")
				}
				if !strings.Contains(styleEscape.ReplaceAllString(sc.lines[z.y-1], ""), "╭─────────╮") || !strings.Contains(styleEscape.ReplaceAllString(sc.lines[z.y+1], ""), "╰─────────╯") {
					t.Fatal("Start must have a compact rounded border")
				}
				marked := strings.Contains(styleEscape.ReplaceAllString(sc.lines[z.y], ""), "▸")
				if marked != (focus == 2) {
					t.Fatal("incorrect Start focus marker")
				}
				t.Logf("focus=%d raw=%q", focus, sc.lines[z.y])
				for y := z.y - 1; y <= z.y+1; y++ {
					t.Log(strings.TrimRight(styleEscape.ReplaceAllString(sc.lines[y], ""), " "))
				}
			}
		}
	}
}

func TestT23SeatArrows(t *testing.T) {
	for _, width := range []int{60, 100, 140} {
		for _, count := range []int{0, 3, 50} {
			m := askingModel(width, 40)
			if count == 0 {
				m.agents = nil
			} else if count == 50 {
				m.agents = manyAgents(50)
			}
			m.focus = focusSeats
			for _, code := range []rune{tea.KeyRight, tea.KeyDown} {
				m, _ = pressKey(m, code, 0)
			}
			want := min(2, max(0, count-1))
			if m.cursor != want || m.focus != focusSeats {
				t.Fatalf("width=%d count=%d cursor=%d focus=%d", width, count, m.cursor, m.focus)
			}
			for _, code := range []rune{tea.KeyLeft, tea.KeyUp, tea.KeyLeft, tea.KeyUp} {
				m, _ = pressKey(m, code, 0)
			}
			if m.cursor != 0 || m.focus != focusSeats {
				t.Fatal("arrows leave seats at boundary")
			}
			if count > 0 {
				m, _ = pressKey(m, tea.KeyRight, 0)
				pane := m.agents[m.cursor].Pane
				before := m.picked[pane]
				m, _ = pressKey(m, tea.KeySpace, 0)
				if m.picked[pane] == before {
					t.Fatal("space does not toggle current seat")
				}
				m, _ = pressKey(m, 'a', 0)
				if m.selectedCount() != count {
					t.Fatal("a does not select all")
				}
				m, _ = pressKey(m, 'n', 0)
				if m.selectedCount() != 0 {
					t.Fatal("n does not clear")
				}
				for i := 0; i < count+2; i++ {
					m, _ = pressKey(m, tea.KeyDown, 0)
				}
				if m.cursor != count-1 || m.focus != focusSeats {
					t.Fatal("bottom boundary leaves seats")
				}
				top, bottom := m.visibleSeats()
				if m.cursor < top || m.cursor >= bottom {
					t.Fatal("arrow navigation leaves cursor offscreen")
				}
			}
		}
	}
}

// Interpret SGR backgrounds in truecolor output instead of only stripping ANSI:
// every painted cell on a button row must belong to an interactive button span.
func TestT23ButtonBackgroundBounds(t *testing.T) {
	buttons := map[string]bool{"welcome-start": true, "welcome-shortcut": true, "ask": true, "settings-shortcut": true, "settings-close": true, "copy": true, "goto": true, "new": true, "judge-now": true, "answer": true, "review": true}
	seen := map[string]bool{}
	for _, w := range []int{60, 100, 140} {
		models := []model{}
		for focus := 0; focus < 3; focus++ {
			m := askingModel(w, 40)
			m.phase = welcoming
			m.welcomeFocus = focus
			models = append(models, m)
			m = askingModel(w, 40)
			m.page = "settings"
			m.settingsFocus = focus
			models = append(models, m)
		}
		m := askingModel(w, 40)
		m.focus = focusAsk
		models = append(models, m)
		m.input.SetValue("")
		models = append(models, m)
		m = watchingModel(w, 40)
		m.run.Seats[1].State = Done
		m.run.Judge.Sent = time.Time{}
		models = append(models, m)
		m.run.Reviews = []*Seat{{State: Done, Answer: "A review"}, {State: Done, Answer: "Another review"}}
		m.seat = 0
		models = append(models, m)
		for _, m := range models {
			sc := m.layout()
			for _, z := range sc.zones {
				if !buttons[z.act] {
					continue
				}
				seen[z.act] = true
				line := sc.lines[z.y]
				x := 0
				bg := false
				for len(line) > 0 {
					if strings.HasPrefix(line, "\x1b[") {
						end := strings.IndexByte(line, 'm')
						if end < 0 {
							t.Fatal("unterminated SGR")
						}
						params := strings.Split(line[2:end], ";")
						for i := 0; i < len(params); i++ {
							n, _ := strconv.Atoi(params[i])
							if n == 0 || n == 49 {
								bg = false
							}
							if n >= 40 && n <= 47 || n >= 100 && n <= 107 {
								bg = true
							}
							if n == 38 || n == 48 || n == 58 {
								if n == 48 {
									bg = true
								}
								if i+1 < len(params) {
									if params[i+1] == "2" {
										i += 4
									} else if params[i+1] == "5" {
										i += 2
									}
								}
							}
						}
						line = line[end+1:]
						continue
					}
					r := []rune(line)[0]
					cellWidth := lipgloss.Width(string(r))
					if bg {
						for cell := x; cell < x+cellWidth; cell++ {
							covered := false
							for _, b := range sc.zones {
								if buttons[b.act] && b.y == z.y && cell >= b.x0 && cell < b.x1 {
									covered = true
								}
							}
							if !covered {
								t.Fatalf("%s width=%d: background outside button at cell %d", z.act, w, cell)
							}
						}
					}
					x += cellWidth
					line = line[len(string(r)):]
				}
				if bg {
					t.Fatal("background remains active at end of row")
				}
			}
		}
	}
	for action := range buttons {
		if !seen[action] {
			t.Errorf("button %s was not audited", action)
		}
	}
}
