package main

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Wrap styled text by visible terminal cells, including long unbroken words.
func wrapBody(body string, width int) string {
	return ansi.Hardwrap(ansi.Wrap(body, max(1, width), ""), max(1, width), true)
}

func markdownBody(source string, width int) string {
	var rows []string
	fenced := false
	for _, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			// Preserve code whitespace and syntax; wrap only at the pane boundary.
			rows = append(rows, ansi.Hardwrap(dim.Render(strings.ReplaceAll(line, "\t", "    ")), max(1, width), true))
			continue
		}
		heading := false
		for _, prefix := range []string{"### ", "## ", "# "} {
			if strings.HasPrefix(line, prefix) {
				line = strings.TrimPrefix(line, prefix)
				heading = true
				break
			}
		}
		if heading && len(rows) > 0 && rows[len(rows)-1] != "" {
			rows = append(rows, "")
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if strings.HasPrefix(line[indent:], "- ") || strings.HasPrefix(line[indent:], "* ") {
			line = line[:indent] + "• " + line[indent+2:]
		}
		rows = append(rows, wrapBody(markdownInline(line, heading), width))
	}
	return strings.Join(rows, "\n")
}

func markdownInline(line string, heading bool) string {
	base := text.Bold(false)
	if heading {
		base = accent.Bold(true)
	}
	var out strings.Builder
	for len(line) > 0 {
		if strings.HasPrefix(line, "`") {
			if end := strings.Index(line[1:], "`"); end >= 0 {
				out.WriteString(colour("#a5d6ff").Bold(false).Render(line[1 : end+1]))
				line = line[end+2:]
				continue
			}
		}
		if strings.HasPrefix(line, "**") {
			if end := strings.Index(line[2:], "**"); end >= 0 {
				out.WriteString(base.Bold(true).Render(line[2 : end+2]))
				line = line[end+4:]
				continue
			}
		}
		next := len(line)
		for _, marker := range []string{"`", "**"} {
			if i := strings.Index(line[1:], marker); i >= 0 && i+1 < next {
				next = i + 1
			}
		}
		out.WriteString(base.Render(line[:next]))
		line = line[next:]
	}
	return out.String()
}
