package main

func askFocusMark(active bool) string {
	if active {
		return accent.Render("▸ ")
	}
	return "  "
}

func paddedCell(value string, width int) string { return fit(value, width) }

type helpEntry struct{ key, meaning string }

var askHelp = []helpEntry{
	{"tab/S-tab", "focus"}, {"↑↓ / k/j", "section / row"},
	{"enter", "use focused"}, {"space/x", "toggle seat"},
	{"a/n", "all idle / none"}, {"←→", "cycle judge"},
	{"J / enter", "judge picker"}, {"space", "peer switch"},
	{"s", "Settings"}, {"?", "help"}, {"esc", "close"},
	{"ctrl+c", "quit"}, {"mouse", "fields / buttons"},
}

var watchHelp = []helpEntry{
	{"←→ / tab", "next seat"}, {"S-tab / h/l", "prev / next"},
	{"1-9", "seat"}, {"[ / ]", "seat page"}, {"v", "verdict"},
	{"r", "answer / review"}, {"↑↓ / wheel", "scroll"},
	{"c", "copy"}, {"f", "agent pane"}, {"j", "judge now"},
	{"n", "new question"}, {"q / esc", "close"}, {"ctrl+c", "quit"},
}

func helpOverlay(base screen, width, height int) screen {
	panelWidth := min(76, width-4)
	inside := panelWidth - 4
	col := (inside - 3) / 2
	var body screen
	body.add(seg(fit(accent.Render("ASK"), col) + " │ " + accent.Render("CURRENT RUN")))
	for i := 0; i < max(len(askHelp), len(watchHelp)); i++ {
		left, right := "", ""
		if i < len(askHelp) {
			left = accent.Render(askHelp[i].key) + " " + dim.Render(askHelp[i].meaning)
		}
		if i < len(watchHelp) {
			right = accent.Render(watchHelp[i].key) + " " + dim.Render(watchHelp[i].meaning)
		}
		body.add(seg(fit(left, col) + " │ " + fit(right, inside-col-3)))
	}
	body.blank()
	body.add(seg(accent.Render("ctrl+left/right") + " " + dim.Render("previous / next page")))
	body.add(seg(dim.Render("Runs: ↑↓ select · enter open · wheel scroll")))
	body.add(seg(dim.Render("Return: Council tab in Herdr · bound shortcut")))
	body.add(act(button.Render("Close"), "help-close"))
	return placeOverlay(base, roundedPanel(body, panelWidth, "COUNCIL / HELP"), width, height)
}
