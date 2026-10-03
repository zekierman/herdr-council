package main

import (
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	artWidth  = 36
	artHeight = 16
)

const artRamp = " .:-=+*#%@"

var councilArt = makeCouncilArt()

// makeCouncilArt samples an abstract seal: two concentric dotted rings and a
// central diamond where converging paths meet. It depicts no fixed seat count.
// Vertical distance is doubled to compensate for terminal cell proportions;
// absolute coordinates guarantee exact horizontal and vertical mirroring.
func makeCouncilArt() []string {
	rows := make([]string, artHeight)
	for y := 0; y < artHeight; y++ {
		var row strings.Builder
		for x := 0; x < artWidth; x++ {
			px := math.Abs(float64(x) - float64(artWidth-1)/2)
			py := math.Abs(float64(y)-float64(artHeight-1)/2) * 2
			r := math.Hypot(px, py)

			luma := 0.08 * math.Exp(-math.Pow(r/11.8, 6))
			luma += 0.54 * gaussian(r-11.5, 1.55)
			luma += 0.29 * gaussian(r-7.65, 1.35)

			// Four soft paths converge continuously into one decision point.
			path := gaussian(py-0.67*px, 1.20) * gaussian(r-5.5, 5.4)
			luma += 0.18 * path
			diamond := px/4.2 + py/5.6
			luma = math.Max(luma, 0.80*gaussian(diamond-1.0, 0.28))
			luma = math.Max(luma, 0.89*gaussian(r, 1.75))
			idx := int(math.Round(math.Min(1, luma) * float64(len(artRamp)-1)))
			row.WriteByte(artRamp[idx])
		}
		rows[y] = row.String()
	}
	return rows
}

func gaussian(distance, sigma float64) float64 {
	v := distance / sigma
	return math.Exp(-0.5 * v * v)
}

// Each density step gets a quiet gray, preserving the dot pattern on black.
var artGreys = [...]string{
	"", "#3a3f47", "#424850", "#4a515a", "#535b65",
	"#5e6873", "#6a7480", "#76818c", "#828c97", "#8b949e",
}

func styledArtRow(row string) string {
	var out strings.Builder
	for _, ch := range row {
		if ch == ' ' {
			out.WriteByte(' ')
			continue
		}
		idx := strings.IndexRune(artRamp, ch)
		out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(artGreys[idx])).Render(string(ch)))
	}
	return out.String()
}

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
