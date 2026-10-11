# Changelog

All notable changes to xplan are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
