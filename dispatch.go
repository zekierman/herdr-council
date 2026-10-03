package main

import tea "charm.land/bubbletea/v2"

// seatSentMsg reports that one seat's prompt went out (or failed).
type seatSentMsg struct {
	run  string
	seat int
	err  error
}

// dispatch saves the run, then returns one command per seat. Bubble Tea runs them concurrently,
// so a large council fans out without freezing the UI, and results come back as seatSentMsg.
func (r *Run) dispatch() tea.Cmd {
	r.markSending()
	cmds := make([]tea.Cmd, len(r.Seats))
	for i, s := range r.Seats {
		i, pane, text, id := i, s.Agent.Pane, r.seatPrompt(s), r.ID
		cmds[i] = func() tea.Msg { return seatSentMsg{id, i, promptAgent(pane, text)} }
	}
	return tea.Batch(cmds...)
}
