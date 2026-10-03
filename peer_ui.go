package main

import (
	"fmt"
	"time"

	"charm.land/lipgloss/v2"
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
	verdictNumber := 2
	line := "1 Answers " + answer
	if peer {
		verdictNumber = 3
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
		line += " · 2 Peer review " + progress
	}
	verdict := "…"
	if r.Judge.State == Done {
		verdict = "✓"
	}
	if r.Judge.State == Failed {
		verdict = "!"
	}
	return fmt.Sprintf("%s · %d Verdict %s", line, verdictNumber, verdict)
}

func (m model) reviewStatus(now time.Time) string {
	if !m.reviewAvailable() {
		return ""
	}
	rv := m.run.Reviews[m.seat]
	return "review " + badge(rv, m.frame, now)
}

func (m model) verdictBody(wrap lipgloss.Style) string {
	body := m.verdictStatus(wrap)
	if ranking := m.run.RankingText(m.run.Judge.State == Done); ranking != "" {
		return bold.Render("PEER RANKING") + "\n" + wrap.Render(ranking) + "\n\n" + body
	}
	return body
}
