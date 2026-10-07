package main

import "charm.land/lipgloss/v2"

const (
	inkHex     = "#e6edf3"
	mutedHex   = "#8b949e"
	faintHex   = "#30363d"
	accentHex  = "#4fc3ff"
	doneHex    = "#2ee88a"
	waitingHex = "#ffd23f"
	failedHex  = "#ff5f56"
	darkHex    = "#0d1117"
)

var (
	dim     = colour(mutedHex)
	text    = colour(inkHex)
	bold    = colour(inkHex).Bold(true)
	accent  = colour(accentHex)
	good    = colour(doneHex)
	warn    = colour(waitingHex)
	bad     = colour(failedHex)
	button  = colour(inkHex).Background(lipgloss.Color(faintHex)).Padding(0, 1)
	primary = colour(darkHex).Background(lipgloss.Color(accentHex)).Bold(true).Padding(0, 1)
	keycap  = colour(inkHex).Background(lipgloss.Color(faintHex)).Padding(0, 1)
	spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

func colour(hex string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)) }

func agentColour(name string) string {
	switch name {
	case "claude":
		return "#d97757"
	case "codex":
		return "#9c9ba8"
	case "agy":
		return "#8cc152"
	case "gemini":
		return "#4796e4"
	case "copilot":
		return "#d1a8ff"
	case "opencode":
		return "#808080"
	case "kimi":
		return "#1e90ff"
	case "qwen":
		return "#ffd700"
	case "cline", "cursor":
		return inkHex
	default:
		return mutedHex
	}
}

func agentName(name string) string {
	if name == "agy" {
		return colour("#f2922e").Render("a") + colour("#8cc152").Render("g") + colour("#4aa3df").Render("y")
	}
	return colour(agentColour(name)).Render(name)
}

func stateMark(s *Seat, frame int) string {
	if s == nil {
		return colour(waitingHex).Render("●")
	}
	if s.Sent.IsZero() && s.State != Done && s.State != Failed && s.State != Silent {
		return warn.Render("●")
	}
	switch s.State {
	case Done:
		return colour(doneHex).Render("●")
	case Working, Sending:
		return colour(accentHex).Render(spinner[frame%len(spinner)])
	case Blocked, Late:
		return colour(waitingHex).Render("!")
	case Failed:
		return colour(failedHex).Render("×")
	default:
		return colour(mutedHex).Render("○")
	}
}
