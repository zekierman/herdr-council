# Council

https://github.com/user-attachments/assets/5e04b781-c49b-4102-8fe3-ec7930d38a13

A [herdr](https://github.com/herdrdev/herdr) plugin. Put one question to every coding agent in your workspace, let each answer on its own, then have a judge agent weigh the answers blind and write one verdict.

1. **Ask.** Write the question once.
2. **Seats answer alone.** Each selected agent (Claude Code, Codex, Antigravity, ...) gets the question in its own pane and writes its answer to its own folder. No seat sees another's answer.
3. **Peer review.** Each agent ranks the other agents' answers, anonymised as A, B, C, and never its own. The rankings are averaged. (On by default for up to six seats; it doubles the time.)
4. **Blind verdict.** A judge agent reads the anonymised answers and the peer ranking, with names hidden and order shuffled, and writes: consensus, conflicts, what was missed or risky, and a final answer. Then the names are revealed.

Different models notice different things. Where they agree you can trust the answer more; where they disagree there is usually a risk worth knowing about. Council works with whatever agents are running in the workspace, two or fifty.

## See it work

A real run in herdr, 30 seconds: three agents review a small `retry.ts`, rank each other's answers, Claude judges blind, and the run lands in **Runs**. The wait for answers is sped up.

https://github.com/user-attachments/assets/678af09f-8d15-4878-bc8a-9a5f1c2fcdc9

## Install

```
herdr plugin install zekierman/herdr-council
```

Requires herdr 0.9 or newer, on Windows, macOS or Linux. The install step downloads a prebuilt binary and checks its SHA-256; if that isn't possible it builds from source with Go.

## Open it

The first time it opens, Council explains itself and asks how you want to come back to it.

- **Click:** Council keeps a tab named **Council** in each workspace. Closing it hides it until the next herdr start; turn this off in Council's settings (⚙).
- **Shortcut:** use "Add shortcut" in the welcome screen or settings, or run `herdr plugin action invoke bind --plugin herdr-council`. It picks a key that is free in both herdr's defaults and your config (usually `prefix+a`), adds it and reloads herdr.
- **Command line:** `herdr plugin action invoke open --plugin herdr-council` opens the popup; `... invoke tab ...` opens the Council tab.

herdr doesn't let plugins add items to its right-click menu, so Council isn't there.

Council looks like a small desktop app: a sidebar with **Ask**, **Runs** and **Settings**, and one content area.

- **Ask:** write the question, pick the agents, choose the judge from a dropdown, turn peer review on or off.
- **Current run:** a list of agents with live status on the left, the selected answer, review or verdict on the right.
- **Runs:** every question asked in this workspace, newest first; open any of them again.

Everything is clickable. With the keyboard, `ctrl+←/→` switches pages, `tab` moves between controls and `?` lists every key. Closing Council doesn't stop anything: the agents keep writing, and reopening it shows the latest question.

## How answers are collected

Each seat is asked to write its answer to a file and end it with a `DONE` line; Council watches the files rather than reading agent screens. If an agent stops at a permission prompt, Council shows it as waiting and "Go to agent" takes you there. Codex in its default sandbox asks before writing outside its workspace (once for its answer, once for its review); approve it, or start Codex with a sandbox that allows Council's state folder.

## License

MIT

Music in the promo video: "Pulsing Ambient Techno" by BudgetPixel AI (https://budgetpixel.com/background-music/pulsing-ambient-techno-dfd07da3), CC BY 4.0; trimmed with volume automation.
