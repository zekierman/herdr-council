package main

import (
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

// The emblem is a balance scale: the judge weighs the answers blind. It is drawn in braille
// dots (2×4 per cell, so lines stay thin and dots come out square) and shows no seat count,
// so it reads the same with two agents or ten.
const (
	artWidth  = 36 // cells
	artHeight = 16
	dotsW     = artWidth * 2
	dotsH     = artHeight * 4
)

var (
	artDots    = drawScale()
	councilArt = toBraille(artDots)
	artInk     = lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
)

// drawScale plots every shape in terms of the distance from the centre line, so the
// left half is an exact mirror of the right.
func drawScale() [][]bool {
	const (
		cx      = float64(dotsW-1) / 2
		armX    = 27.5 // where the chains hang from, measured from the centre
		panY    = 34.0
		panRx   = 7.5
		panRy   = 4.5
		beamTop = 11.0
	)
	g := make([][]bool, dotsH)
	for y := range g {
		g[y] = make([]bool, dotsW)
		fy := float64(y)
		for x := range g[y] {
			dx := math.Abs(float64(x) - cx)
			on := math.Hypot(dx, fy-5) <= 2.6 || // finial
				(dx <= 0.6 && fy >= 7 && fy <= 55) || // pillar
				(fy >= beamTop && fy <= beamTop+1 && dx <= armX) || // beam
				math.Hypot(dx-armX, fy-beamTop-0.5) <= 1.8 || // beam ends
				segDist(dx, fy, armX, beamTop+1, armX-panRx, panY) <= 0.55 || // chains
				segDist(dx, fy, armX, beamTop+1, armX+panRx, panY) <= 0.55 ||
				(fy >= panY && sq((dx-armX)/panRx)+sq((fy-panY)/panRy) <= 1) || // pans
				(fy >= 50 && fy <= 57 && dx <= 1.5+(fy-50)*1.4) || // foot
				(fy >= 58 && fy <= 60 && dx <= 14) // plinth
			g[y][x] = on
		}
	}
	return g
}

func sq(v float64) float64 { return v * v }

// segDist is the distance from (px, py) to the segment (ax, ay)-(bx, by).
func segDist(px, py, ax, ay, bx, by float64) float64 {
	vx, vy := bx-ax, by-ay
	t := ((px-ax)*vx + (py-ay)*vy) / (vx*vx + vy*vy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(px-(ax+t*vx), py-(ay+t*vy))
}

// toBraille packs each 2×4 block of dots into one braille character (U+2800 + dot bits).
func toBraille(g [][]bool) []string {
	bit := [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}
	rows := make([]string, artHeight)
	for cy := 0; cy < artHeight; cy++ {
		var b strings.Builder
		for cx := 0; cx < artWidth; cx++ {
			var r rune
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					if g[cy*4+dy][cx*2+dx] {
						r |= bit[dy][dx]
					}
				}
			}
			if r == 0 {
				b.WriteRune(' ')
			} else {
				b.WriteRune(0x2800 + r)
			}
		}
		rows[cy] = b.String()
	}
	return rows
}

func styledArtRow(row string) string { return artInk.Render(row) }

func (m model) artPosition() (int, bool) {
	if m.phase != asking || m.w < 88 || m.h < 28 {
		return 0, false
	}
	return m.w - artWidth - 4, true
}

// Art is static; appending it after the interactive left column leaves every
// screen.add/raw click zone at its original x coordinate.
func placeCouncilArt(sc *screen, x int) {
	for y, row := range councilArt {
		if y >= len(sc.lines) {
			break
		}
		pad := x - lipgloss.Width(sc.lines[y])
		if pad < 0 {
			continue
		}
		sc.lines[y] += strings.Repeat(" ", pad) + styledArtRow(row)
	}
}
