package main

import "github.com/charmbracelet/x/ansi"

// shorten counts terminal cells and preserves ANSI styles and wide Unicode characters.
func shorten(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}
