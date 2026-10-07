package main

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const markdownFixture = "# Decision\nNormal body with **strong** and `retry()` text.\n## Reasons\n- Clear modules\n* Independent tests\n1. Keep numbered items\n### Example\n```go\n  if ready {\n    **literal** `literal`\n  }\n```\nPlain ending."

func TestMarkdownRendering(t *testing.T) {
	body := markdownBody(markdownFixture, 100)
	want := "Decision\nNormal body with strong and retry() text.\n\nReasons\n• Clear modules\n• Independent tests\n1. Keep numbered items\n\nExample\n  if ready {\n    **literal** `literal`\n  }\nPlain ending."
	if got := ansi.Strip(body); got != want {
		t.Fatalf("rendered Markdown:\n%s\nwant:\n%s", got, want)
	}
	for _, styled := range []string{accent.Bold(true).Render("Decision"), text.Bold(false).Render("Normal body with "), text.Bold(true).Render("strong"), colour("#a5d6ff").Bold(false).Render("retry()"), dim.Render("    **literal** `literal`")} {
		if !strings.Contains(body, styled) {
			t.Errorf("missing styled span %q", styled)
		}
	}
	if got := ansi.Strip(markdownBody("Unmatched ** and ` stay.\n#### Not supported\n\n## Heading", 100)); got != "Unmatched ** and ` stay.\n#### Not supported\n\nHeading" {
		t.Fatalf("unmatched syntax/blank line: %q", got)
	}
}

func TestMarkdownPaneWrappingAndCopy(t *testing.T) {
	oldWrite := writeClipboard
	t.Cleanup(func() { writeClipboard = oldWrite })
	var copied string
	writeClipboard = func(s string) error { copied = s; return nil }
	long := markdownFixture + "\n**" + strings.Repeat("界", 70) + "** `" + strings.Repeat("identifier", 20) + "`\n```\n  " + strings.Repeat("x", 240) + "\n```"
	for _, width := range []int{60, 100, 140} {
		m := peerWatchModel(width, 40)
		for _, view := range []string{"answer", "review", "verdict"} {
			m.seat = 0
			m.showReview = false
			switch view {
			case "answer":
				m.run.Seats[0].State = Done
				m.run.Seats[0].Answer = long
			case "review":
				m.showReview = true
				m.run.Reviews[0].State = Done
				m.run.Reviews[0].Answer = long
			case "verdict":
				m.seat = len(m.run.Seats)
				m.run.Judge.State = Done
				m.run.Judge.Answer = long
			}
			m.refreshAnswer()
			body := markdownBody(long, m.answer.Width())
			if view == "verdict" {
				body = m.verdictBody(m.answer.Width())
				plain := ansi.Strip(body)
				if !strings.Contains(plain, "PEER RANKING") || !strings.Contains(plain, "Revealed:") {
					t.Fatal("ranking/reveal missing")
				}
			}
			for _, line := range strings.Split(body, "\n") {
				if got := lipgloss.Width(line); got > m.answer.Width() {
					t.Fatalf("%s terminal=%d line=%d pane=%d: %q", view, width, got, m.answer.Width(), line)
				}
			}
			assertScreen(t, m)
			copied = ""
			m, _ = pressKey(m, 'c', 0)
			if view == "verdict" {
				if !strings.Contains(copied, long) || copied != m.currentText() {
					t.Fatal("verdict copy changed original Markdown")
				}
			} else if copied != long {
				t.Fatal("copy did not return original Markdown")
			}
			if strings.Contains(copied, "\x1b[") {
				t.Fatal("copy contains styling")
			}
		}
	}
}

func TestMarkdownSnapshot(t *testing.T) {
	t.Log("BEFORE:\n" + markdownFixture)
	t.Log("AFTER (ANSI stripped, width 64):\n" + ansi.Strip(markdownBody(markdownFixture, 64)))
	// The verdict row uses ◆: ★ is drawn double-width by fallback fonts and shifts the line.
	m := watchingModel(140, 40)
	if !strings.Contains(ansi.Strip(strings.Join(m.layout().lines, "\n")), "◆ Verdict") {
		t.Fatal("verdict diamond changed")
	}
}
