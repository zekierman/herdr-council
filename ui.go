package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// shorten works in terminal cells, including wide Unicode characters.
func shorten(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		cw := lipgloss.Width(string(r))
		if used+cw > width-1 {
			break
		}
		b.WriteRune(r)
		used += cw
	}
	return b.String() + "…"
}

func verdictState(s *Seat) string {
	if s == nil || s.Sent.IsZero() {
		return "waiting for answers"
	}
	switch s.State {
	case Done:
		return "ready to read"
	case Working, Sending:
		return "judge is weighing answers"
	case Blocked:
		return "judge needs attention"
	default:
		return "judge could not finish"
	}
}

// tabRows keeps one compact strip and a fixed verdict tab, regardless of seat count.
func (m model) tabRows(now time.Time, width int) [][][2]string {
	_, _ = now, width
	start, end := m.tabWindow()
	row := [][2]string{seg(" ")}
	if start > 0 {
		row = append(row, act(dim.Render(fmt.Sprintf("‹ %d more", start)), "tab-prev"), seg(" "))
	}
	for i := start; i < end; i++ {
		row = append(row, act(compactTab(m.run.Seats[i], i, m.seat), fmt.Sprintf("seat:%d", i)), seg(" "))
	}
	if end < len(m.run.Seats) {
		row = append(row, act(dim.Render(fmt.Sprintf("%d more ›", len(m.run.Seats)-end)), "tab-next"), seg(" "))
	}
	v := dim.Render("[VERDICT]")
	if m.onVerdict() {
		v = accent.Render("[VERDICT]")
	}
	row = append(row, act(v, fmt.Sprintf("seat:%d", len(m.run.Seats))))
	return [][][2]string{row}
}
