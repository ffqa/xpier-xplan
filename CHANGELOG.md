# Changelog

All notable changes to xplan are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.5.0] - 2026-10-14

### Added

- **Agent memory over MCP** at `http://127.0.0.1:7788/mcp`. Eight tools:
  `xplan_projects`, `xplan_context`, `xplan_park`, `xplan_note`,
  `xplan_propose_adr`, `xplan_inbox`, `xplan_start_review`, `xplan_get_review`.
  Agents pass their working directory and xplan finds the project. No tool can
  change the intent; proposed ADRs stay `proposed` until the owner decides.
  `xplan mcp` is a stdio bridge for agents without HTTP MCP. Settings → Agent
  memory (MCP) has copy-ready setup for Cursor, Claude Code, Codex and others.
- **Background service**: `xplan service install|uninstall|restart|status`
  (launchd on macOS, systemd user unit on Linux) so scheduled reviews,
  notifications and MCP keep running. `xplan serve --log-file PATH` writes a log
  rotated at 5 MB.
- **Notifications** when a review needs attention (or only when critical) or
  fails: desktop (macOS, Linux) and a chat webhook in the right format for
  Slack, Feishu/Lark, DingTalk, WeCom, Discord or plain JSON. Settings →
  Notifications with a test button; `POST /api/notify/test`. The webhook URL
  is never returned by the API.
- Drift reviews see the other registered projects (summary, goal, stack, last
  activity) and must name the project a borrowed concept seems to come from.
- Release pipeline: CI on Linux and macOS, tagged releases for macOS, Linux and
  Windows (amd64 and arm64) with checksums, and `scripts/install.sh`.
- MIT `LICENSE` and `web/package-lock.json`.

### Changed

- Windows builds: agents are found through `PATHEXT`, and a review is stopped
  with `taskkill /T`.
- Agent search paths cover nvm, fnm, Volta, Bun, Go and Homebrew on Linux, not
  only this machine's setup.
- `/open` falls back to VS Code or the system file manager; `/xteam` is listed
  only when xteam is installed.
- A binary built without the web UI (for example with `go install`) explains
  where to get a full build.
- Links in notifications and the MCP address use `127.0.0.1` and follow
  `serve --listen`.

### Fixed

- `npm run build` no longer deletes `web/dist/.gitkeep`, which fresh clones need
  to compile.

## [0.4.0] - 2026-10-13

### Added

- Per-reviewer **model**, **agent** and **timeout**. Commands take the new
  `{model}` and `{agent}` placeholders; an argument holding an empty one is
  dropped, so the agent keeps its own default. Every built-in preset now has a
  model setting; `claude`, `opencode` and `kiro` also have an agent setting.
- Settings → Reviewer agents: edit any reviewer, add your own (any CLI agent),
  reset a changed preset, delete a custom one. API:
  `PUT /api/reviewers/{name}` and `DELETE /api/reviewers/{name}`.
- Templates for third-party agents: `codebuddy` (Tencent CodeBuddy Code),
  `qodercli` (Qoder CLI) and `claude-provider` (Claude Code on any
  Anthropic-compatible provider through a settings file).
- Built-in `devin` preset (Devin CLI print mode, verified read-only).
- `/api/meta` reviewers report `command`, `model`, `agent`, `timeout`,
  `effective_timeout`, `supports_model`, `supports_agent`, `builtin` and
  `modified`, plus `reviewer_templates`.

### Changed

- Timeouts explain themselves: "No answer within 20m…" with likely causes
  (rate-limited model, quiet retries, big project), what to change, and the
  agent's last output lines.
- `omp` starts without skills, extensions, rules, LSP and title generation
  (smoke test about 30 s → 5 s).
- `config.yaml` stores only the fields you changed on built-in presets, so
  preset fixes keep reaching you. Built-in commands written in full by
  v0.1–v0.3 are dropped on load; commands you edited are kept.
- Built-in preset descriptions are translated in the Chinese UI.

### Fixed

- An agent that exits 0 with no output, or with only a short login message
  (seen with `codebuddy` and `qodercli` when logged out), counts as failed, and
  `auto` moves on to the next reviewer.
- Saved health details from older versions no longer show terminal color codes.
- Editing, resetting or deleting a reviewer forgets its old health result.

## [0.3.0] - 2026-10-12

### Added

- Simplified Chinese: the whole UI plus console replies, health flags,
  reviewer status, review kind names and job errors. Switch with the status
  bar button or Settings → Language; the choice is saved as `language` in
  `config.yaml` and shared by every browser.
- Report language presets (English, Simplified Chinese) in Settings.
- Reviewer presets for `kimi`, `gemini`, `qwen`, `opencode` and `kiro`, each
  in a read-only mode (kimi uses a generated Read/Grep/Glob-only agent profile
  under `~/.xplan/agents/`). New `{xplan_home}` placeholder.
- `auto` reviewer, now the default: picks the first working preset in
  `reviewer_order`, skips presets that are missing or not logged in, and moves
  on when a real run hits a login wall. Jobs record `reviewer_requested` and
  the skipped presets in `attempts`.
- Reviewer health (`~/.xplan/reviewers-health.json`) from smoke tests and real
  runs, with `POST /api/reviewers/{name}/test` and `POST /api/reviewers/test`.
  Settings → Reviewer agents shows health, the last check, Test / Test all and
  reorder arrows; the status bar shows `auto → <pick>`.
- `npm run i18n:check` and a Go catalog test that fail on untranslated strings.

### Changed

- The `claude` preset adds a hard tool allowlist (`--tools=Read,Grep,Glob`) and
  `--strict-mcp-config`, so tools allowed in the user's own Claude settings or
  MCP servers cannot write. The previous built-in command is upgraded on load.
- Go module path is `github.com/ffqa/xpier-xplan`.

### Fixed

- Reviewer error details no longer contain terminal color codes.

## [0.2.0] - 2026-10-11

### Added

- Intent drafts: an `intent` job lets the reviewer reconstruct why an existing
  project was started from its README, docs and earliest commits. The draft
  fills an empty `intent.md` with a `> **Draft**` line for the owner to correct;
  a written intent is never overwritten (the draft stays in the job report and
  the Intent tab offers "Load into editor").
- "Draft missing intents" on the Overview and `POST /api/intent-drafts`: drafts
  only for projects whose first commit predates their registration by more
  than two days, so new projects stay owner-written.
- `/draft-intent` console command, "Draft intent from history" in ⌘K, and draft
  buttons in the Brief card and the Intent tab.
- `intent_draft` health flag; drift reviews treat a draft intent as
  low-confidence and say which findings depend on it.
- `GET/PUT /api/projects/{slug}/intent` return `filled` and `draft`; git scans
  record the first commit date and total commit count.

### Changed

- Only review kinds (drift, code, security, deploy, test) produce verdicts,
  appear in the Review menu, count towards health and can be scheduled.

### Fixed

- The Markdown editor no longer replaces unsaved typing when the file changes
  on the server (for example when an intent draft lands), and the Intent
  editor no longer carries unsaved text over to another project.

## [0.1.0] - 2026-10-11

### Added

- Project registry under `~/.xplan`: `project.yaml`, xplan-owned `intent.md`,
  ADRs, journal (park notes, notes), visit state, scan history.
- Scanner: git (branch, HEAD, recent commits, dirty files), docs classified by
  kind (readme, changelog, spec, adr, progress, agent, xteam), checklist
  progress, `.xteam/tasks` slice stages, stack detection.
- Health flags: missing intent, stale "active" projects, dirty trees, missing
  park note, review verdicts, failed scheduled reviews.
- Review engine: drift, code, security, deploy, test and ask reviews through
  read-only agent presets (`cursor-agent`, `codex`, `claude`, `omp`), JSON
  verdict parsing, working-tree guard, cancellation, concurrency limit,
  per-project schedules (`off`, `daily`, `weekly`).
- Console: `/review`, `/ask`, `/park`, `/note`, `/adr`, `/status`, `/scan`,
  `/caught-up`, `/xteam`, `/open`, `/help`; plain text asks the reviewer.
- HTTP API with local-only guards (loopback Host, `X-Xplan` header, sensitive
  file blocklist) and HTTP tests.
- IDE-style web UI embedded in the binary: project sidebar, overview, board
  with drag and drop, inbox, re-entry brief, intent editor, decisions, docs
  explorer (by kind or file tree), reviews with findings, journal, per-project
  and global settings, command palette (⌘K), console (⌘J).
- Report file links (`/abs/project/path:line`) open the file in the Docs tab.
- CLI: `serve`, `add`, `discover`, `list`, `scan`, `brief`, `park`, `review`,
  `version`.

### Fixed

- `claude` preset: the variadic `--allowedTools` flag swallowed the prompt; the
  flags now use `=` and old configs are migrated automatically.
- `omp` preset referenced tools that do not exist (`find`, `ls`); it now uses
  `read,grep,glob`, and old configs are migrated.
- README summaries no longer contain Markdown links, HTML tags or badge/navigation
  lines.
