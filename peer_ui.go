package main

import (
	"fmt"
	"strings"
)

func (m *model) syncPeerDefault() {
	if !m.peerChosen {
		m.peerReview = m.selectedCount() <= 6
	}
}

func (m model) reviewAvailable() bool {
	return m.run != nil && !m.onVerdict() && m.run.reviewsStarted() &&
		m.seat >= 0 && m.seat < len(m.run.Reviews) && m.run.Reviews[m.seat] != nil
}

func (m model) stageLine() string {
	r := m.run
	if r == nil {
		return ""
	}
	answer := fmt.Sprintf("%d/%d", r.answered(), len(r.Seats))
	if r.seatsSettled() {
		answer = "✓"
	}
	peer := r.PeerReview && (r.answered() >= minAnswersForReview || r.reviewsStarted())
	line := "Answers " + answer
	if peer {
		progress := "…"
		if r.reviewsStarted() {
			done, total := 0, 0
			for _, rv := range r.Reviews {
				if rv != nil {
					total++
					if rv.State.settled() {
						done++
					}
				}
			}
			progress = fmt.Sprintf("%d/%d", done, total)
			if r.reviewsSettled() {
				progress = "✓"
			}
		}
		line += " · Peer review " + progress
	}
	verdict := "…"
	if r.Judge != nil && r.Judge.State == Done {
		verdict = "✓"
	}
	if r.Judge != nil && r.Judge.State == Failed {
		verdict = "!"
	}
	if r.Judge == nil {
		verdict = "—"
	}
	return fmt.Sprintf("%s · Verdict %s", line, verdict)
}

func (m model) verdictBody(width int) string {
	body := m.verdictStatus(width)
	ready := m.run.Judge != nil && m.run.Judge.State == Done
	if !ready {
		body = wrapBody(body, width)
	}
	if ready {
		if reveal := m.run.Reveal(); len(reveal) > 0 {
			body += "\n\n" + wrapBody(dim.Render("Revealed: "+strings.Join(reveal, " · ")), width)
		}
	}
	if ranking := m.run.RankingText(ready); ranking != "" {
		var rows []string
		rows = append(rows, dim.Render("  Answer    Avg   Reviews"))
		for i, row := range m.run.Ranking {
			label := fmt.Sprintf("%d. %-7s %4.1f   %d", i+1, "Answer "+row.Letter, row.Avg, row.Votes)
			if ready {
				label += "  (" + m.run.Seats[row.Seat].Agent.Name + ")"
			}
			rows = append(rows, text.Bold(false).Render(label))
		}
		return accent.Bold(true).Render("PEER RANKING") + "\n" + wrapBody(strings.Join(rows, "\n"), width) + "\n\n" + body
	}
	return body
}
