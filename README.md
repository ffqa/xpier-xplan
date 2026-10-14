# xplan

A cockpit that sits **outside** every project you build in parallel.

When you hop between five projects while agents work, two things break: you forget
_why_ a project exists, and ideas from the other four leak into it. xplan keeps
each project's intent and decisions in its own home (`~/.xplan`), collects what the
project writes about itself (README, specs, ADRs, CHANGELOG, checklists, `.xteam/`
slices, git activity), shows everything on an IDE-style board, and periodically
asks a **read-only** agent to review the project from the outside.

- **Projects on the left, files and reviews on the right**, plus a console at the bottom.
- **Re-entry brief**: intent → where you stopped → what changed since → health.
- **External reviews**: drift (is the project still doing what it was started for?),
  code, security, deploy, test, or a free-form question.
- **Intent drafts for existing projects**: you write the intent of a new project
  yourself; for one that existed before xplan, the reviewer reconstructs it from
  the README and git history and you correct it. A written intent is never overwritten.
- **Any agent you have**: cursor-agent, codex, claude, kimi, gemini, qwen,
  opencode, kiro, devin or omp built in, templates for codebuddy, qodercli and
  Claude Code on other providers, or any CLI agent you add yourself. Each
  reviewer can have its own model, agent and timeout. The default `auto`
  reviewer uses the first one that is installed and logged in, and Settings can
  test each one.
- **English or 简体中文**: one click in the status bar switches the UI and the
  server messages; reports can be written in Chinese too.
- **Read-only by design**: xplan never writes into a project, agents run in their
  read-only modes, and a guard flags any working-tree change made during a review.
- **Agent memory (MCP)**: coding agents read a project's intent, decisions, park
  notes and findings at `http://127.0.0.1:7788/mcp`, and leave park notes, notes
  and proposed decisions. They cannot change the intent.
- **Runs in the background**: `xplan service install` keeps it running at login,
  so scheduled reviews run, and it notifies you (desktop or Slack, Feishu,
  DingTalk, WeCom, Discord) when a review needs attention.
- **One binary**: Go backend with the React UI embedded. State is plain files.
  macOS and Linux; Windows is experimental.

See [docs/design.md](docs/design.md) for the design and flowcharts, and
[Fetures.md](Fetures.md) for every feature, API and parameter.

## Quick start

```bash
curl -fsSL https://raw.githubusercontent.com/ffqa/xpier-xplan/main/scripts/install.sh | sh
xplan service install          # keep it running at login (launchd / systemd --user)
xplan add ~/code/my-app        # register a project (or: xplan discover ~/code --add)
```

Then open `http://localhost:7788`. Windows: download the zip from
[Releases](https://github.com/ffqa/xpier-xplan/releases) and run `xplan serve`.

The binaries are not code-signed. The install script is not affected, but a
macOS archive downloaded with a browser is quarantined; clear it with
`xattr -d com.apple.quarantine ./xplan`. Windows SmartScreen may ask once too.

From a clone: `make build && ./xplan serve --open`.

Some embedded browsers render an empty page for `127.0.0.1`, while `localhost`
works.

## Connect your coding agents

Settings → Agent memory (MCP) shows copy-ready setup. In short:

```jsonc
// Cursor: ~/.cursor/mcp.json
{ "mcpServers": { "xplan": { "url": "http://127.0.0.1:7788/mcp" } } }
```

```bash
claude mcp add --transport http --scope user xplan http://127.0.0.1:7788/mcp   # Claude Code
```

Agents that only start commands use the stdio bridge: `xplan mcp`. Add a line
to your agent rules so they use it: call `xplan_context` with the working
directory at the start, and `xplan_park` before stopping.

## Requirements

| Tool         | Version       | Why                                                                                                                          |
| ------------ | ------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Go           | 1.24+         | Backend, uses the 1.22+ `net/http` router                                                                                    |
| Node         | 24 (`.nvmrc`) | Building the UI                                                                                                              |
| An agent CLI | any one       | Reviews: `cursor-agent`, `codex`, `claude`, `kimi`, `gemini`, `qwen`, `opencode`, `kiro-cli`, `devin`, `omp`, or one you add |

The agent CLI must already be logged in (for example `cursor-agent login`,
`codex login`, `opencode auth login`). Settings → Reviewer agents → Test all
shows which ones work on this machine; `auto` skips the rest.

If a reviewer times out, open Settings → Reviewer agents → Edit and pick a
faster model or give that reviewer a longer timeout (for example `40m`).

## Commands

```text
xplan [--home DIR] <command> [args]

serve [--listen ADDR] [--no-schedule] [--open] [--log-file PATH]
                                                  Start the web UI (default 127.0.0.1:7788)
service install|uninstall|restart|status          Run serve in the background at login
mcp [--url URL]                                   Stdio bridge to the running server's MCP endpoint
add <path> [--name NAME]                          Register a project directory
discover <root> [--add]                           List (or register) git repos under root
list                                              List registered projects
scan [slug]                                       Rescan one project, or all
brief <slug>                                      Print the re-entry brief
park <slug> "<stopped> | <next> | <open>"         Leave a park note
review <slug> <kind> [--question Q]               Run a read-only review and wait for it
version                                           Print version
```

Home defaults to `$XPLAN_HOME`, then `~/.xplan`.

## Development

```bash
make dev    # Go backend on :7788 + Vite dev server on :5178 (proxies /api)
make test   # Go HTTP API tests (temp home, status codes and response shape)
make web    # npm ci if needed, type-check and build the UI into web/dist
cd web && npm run i18n:check   # every t("...") string has a Chinese entry
```

Translations: web strings in `web/src/i18n/zh/*.ts`, server strings in
`internal/i18n/zh.go`. The English text is the key.

After rebuilding while the service runs: `xplan service restart`.

Releases: bump `Version` in `internal/app/app.go` and `web/package.json`, add a
`CHANGELOG.md` section, then push a tag `vX.Y.Z`. The release workflow checks
both, runs the tests and publishes binaries for macOS, Linux and Windows.

## Layout

```text
cmd/xplan/          CLI entry point
internal/config     ~/.xplan/config.yaml, reviewer presets
internal/i18n       server message catalog (en, zh)
internal/project    registry: project.yaml, intent.md, ADRs, journal, state
internal/scanner    git, docs, checklists, .xteam slices, stack detection
internal/insight    health flags, overview, brief, inbox
internal/review     prompts, agent runner, auto pick and health, job store, tree guard
internal/console    slash commands and free-form questions
internal/mcp        MCP endpoint: JSON-RPC, project lookup, tools
internal/notify     desktop and chat webhook notifications
internal/service    launchd / systemd user service
internal/desktop    open URLs and files, desktop notifications per OS
internal/server     HTTP API + embedded SPA + local-only guards
web/                Vite + React 19 + Tailwind v4 + shadcn-style components
scripts/install.sh  release installer for macOS and Linux
```

## Security

- Listens on `127.0.0.1` only. Non-GET requests need the `X-Xplan: 1` header and a
  loopback `Host`, which blocks cross-site posts and DNS rebinding.
- File reads are confined to the project root (after resolving symlinks).
  `.env*`, keys, certificates, `secrets/` and `.ssh/` are never served.
- Agents are started with an argv list (no shell) and a timeout.
- `/mcp` cannot require `X-Xplan` (MCP clients do not send it), so it refuses
  any browser `Origin` other than localhost instead, besides the loopback
  `Host` check.
- The notification webhook URL is a secret: it stays in `config.yaml` and the
  API only returns a masked hint.

## License

MIT, see [LICENSE](LICENSE).
