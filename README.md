# Council

A [herdr](https://github.com/herdrdev/herdr) plugin. Put one question to every coding agent in your workspace, let each answer on its own, then have a judge agent weigh the answers blind and write one verdict.

1. **Ask.** Write the question once.
2. **Seats answer alone.** Each selected agent (Claude Code, Codex, Antigravity, ...) gets the question in its own pane and writes its answer to its own folder. No seat sees another's answer.
3. **Blind verdict.** A judge agent reads the answers as A, B, C, with names hidden and order shuffled, and writes: consensus, conflicts, what was missed or risky, and a final answer. Then the names are revealed.

Different models notice different things. Where they agree you can trust the answer more; where they disagree there is usually a risk worth knowing about. Council works with whatever agents are running in the workspace, two or fifty.

## Install

```
herdr plugin install zekierman/herdr-council
```

Requires herdr 0.9 or newer, on Windows, macOS or Linux. The install step downloads a prebuilt binary and checks its SHA-256; if that isn't possible it builds from source with Go.

## Open it

The first time it opens, Council explains itself and asks how you want to come back to it.

- **Click:** Council keeps a tab named **Council** in each workspace. Closing it hides it until the next herdr start; turn this off in Council's settings (⚙).
- **Shortcut:** use "Add shortcut" in the welcome screen or settings, or run `herdr plugin action invoke bind --plugin herdr-council`. It picks a key that is free in both herdr's defaults and your config (usually `prefix+a`), adds it and reloads herdr.
- **Command line:** `herdr plugin action invoke tab --plugin herdr-council`

herdr doesn't let plugins add items to its right-click menu, so Council isn't there.

Inside Council, `tab` moves between sections and `?` lists every key. Everything is clickable too. Closing the popup doesn't stop anything: the agents keep writing, and reopening Council shows the latest question.

## How answers are collected

Each seat is asked to write its answer to a file and end it with a `DONE` line; Council watches the files rather than reading agent screens. If an agent stops at a permission prompt, Council shows it as waiting and "Go to agent" takes you there.

## License

MIT
