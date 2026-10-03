# Council

A [herdr](https://github.com/herdrdev/herdr) plugin. Put one question to every coding agent in your workspace, let each answer on its own, then have a judge agent weigh the answers blind and write one verdict.

1. **Ask.** Write the question once.
2. **Seats answer alone.** Each selected agent (Claude Code, Codex, Antigravity, ...) gets the question in its own pane and writes its answer to its own folder. No seat sees another's answer.
3. **Blind verdict.** A judge agent reads the answers as A, B, C, with names hidden and order shuffled, and writes: consensus, conflicts, what was missed or risky, and a final answer. Then the names are revealed.

Different models notice different things. Where they agree you can trust the answer more; where they disagree there is usually a risk worth knowing about.

## Install

```
herdr plugin install zekierman/herdr-council
```

Requires herdr 0.9 or newer. Early version: building needs Go 1.27; prebuilt binaries are coming.

## Open it

- **Click:** run the "Open the Council tab" action once; a tab named Council stays in your workspace. `council --auto-tab on` opens it in every workspace when herdr starts.
- **Shortcut:** `council --bind` picks a key that is free in both herdr's defaults and your config, adds it, and reloads herdr (default suggestion: `prefix+a`).
- **Command line:** `herdr plugin action invoke open --plugin herdr-council`

Closing the popup doesn't stop anything: the agents keep writing, and reopening Council shows the latest question.

## How answers are collected

Each seat is asked to write its answer to a file and end it with a `DONE` line; Council watches the files rather than reading agent screens. If an agent stops at a permission prompt, Council shows it as waiting and "Go to agent" takes you there.

## License

MIT
