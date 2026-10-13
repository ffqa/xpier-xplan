# Error Diary

Recurring problems and how they were resolved. Newest first.

## 2026-10-13

### "timed out after 20m0s / Working..." on omp

- **Symptom**: drift reviews on omp failed after exactly 20 minutes; the only
  output was "Working...".
- **Cause**: omp's log showed its free model answering
  `429 Endpoint is unavailable`, and omp retried quietly until xplan killed it.
  A replay with a healthy model still needed about 21 minutes on that project.
- **Fix (v0.4.0)**: per-reviewer model and timeout, omp starts without skills,
  extensions, rules and LSP, and the timeout error now explains the likely
  causes and shows the agent's last lines.
- **Lesson**: a timeout is a symptom; read the agent's own log for 429 / 5xx
  retries before raising the limit.

### Logged-out agents that exit with code 0

- **Symptom**: `codebuddy` and `qodercli` "succeeded" with an empty or
  one-line answer.
- **Cause**: both print a short login hint (or nothing) and exit 0 when not
  logged in.
- **Fix**: empty output, or a short output that matches the login pattern,
  now counts as a failure, so `auto` moves on.

### Build in the same batch as its edit (third time)

- **Symptom**: `undefined: fillFromPreset` right after adding the function.
- **Cause**: the edit and `go build` were sent together again.
- **Fix**: same rule as 2026-10-12. Edits first; build and test only after
  every edit result is back.

### Background server died with the shell

- **Symptom**: `nohup ./xplan serve &` stopped answering after the command
  finished.
- **Fix**: start it as its own background shell with
  `exec ./xplan serve --no-schedule`, and stop it by port
  (`lsof -nP -t -iTCP:7788 -sTCP:LISTEN`).

### Markdown table rows not found after an edit (seen again)

- **Symptom**: edits to two rows of the settings table in `Fetures.md` failed
  with "string not found".
- **Cause**: the first edit in the batch widened a column; the formatter
  re-padded the whole table before the next edit ran.
- **Fix**: edit one table at a time and re-read it before the next row.

## 2026-10-12

### Headless `claude -p` hangs: cause found

- **Symptom**: same as before; the smoke test times out after 2 minutes even
  with `--strict-mcp-config`.
- **Cause**: `~/.claude/debug/<session>.txt` (written with `--debug`) shows
  the request going out, no first byte for 30 s, then
  `API error (attempt 1/11): 502 Bad gateway`. The configured API endpoint was
  down, and Claude Code retries up to 11 times. Hooks and MCP were not the
  problem.
- **Lesson**: for a silent agent, read its own debug log before changing flags.
  xplan reports this as `timeout`; the detail cannot show the 502 because
  Claude Code prints nothing to stdout or stderr.

### Old `gemini` and `qwen` picked from the Node 18 folder

- **Symptom**: gemini failed with `Unknown arguments: approval-mode`; qwen
  timed out.
- **Cause**: `LookupAgent` also searches every nvm Node version. The only
  installs were gemini-cli 0.1.9 and qwen-code 0.1.2 under Node 18, too old for
  the preset flags (and they crash when run with Node 18 itself).
- **Fix**: reinstall under the current Node:
  `npm i -g @google/gemini-cli @qwen-code/qwen-code`.

### Parallel edits to the same file (seen again)

- **Symptom**: `undefined: strings` in a test right after adding the import.
- **Cause**: the import edit and `go test` went out in the same parallel
  batch; the test compiled the old file.
- **Fix**: same rule as below, extended: a build or test that depends on an
  edit runs in the next step, never in the same batch.

## 2026-10-11

### Agent presets that look right but do not run

- **Symptom**: `claude` reviews never received the prompt; `omp` exited with
  "unknown tool".
- **Cause**: `--allowedTools` / `--disallowedTools` are variadic in Claude Code
  and consumed the trailing prompt argument. `omp` has no `find` or `ls` tools.
- **Fix**: `--allowedTools=Read,Grep,Glob` form with the prompt right after
  `-p`; `omp --tools read,grep,glob`. `config.isLegacyPreset` rewrites the old
  argv in existing `config.yaml` files.
- **Lesson**: smoke-test every preset with the real binary and a one-line prompt.

### Headless `claude -p` hangs

- **Symptom**: the process never prints and times out.
- **Tried**: `--strict-mcp-config`, `--settings '{"disableAllHooks":true}'`
  (still hangs); `--setting-sources project,local` (loses env-based auth).
- **Status**: solved on 2026-10-12, see above (API endpoint returned 502).

### `cursor-agent` authentication

- **Symptom**: `Authentication required. Please run 'agent login' first`.
- **Fix**: run `cursor-agent login` once in a terminal, or export
  `CURSOR_API_KEY`.
- **Seen again (v0.2.0)**: `cursor-agent status` printed "Login successful!
  Logged in (unable to fetch user details)", yet every run still failed with
  the same error within 5 seconds. `status` is not proof; check with
  `cursor-agent -p --mode ask "Reply with: pong"`. All four bulk intent drafts
  on `cursor-agent` failed; they were rerun on `codex`.

### Parallel edits to the same file

- **Symptom**: several edits sent to one file at once came back with altered
  text or "string not found".
- **Cause**: each edit sees the file before the others land, and the Prettier
  hook reflows Markdown tables and TypeScript after every write.
- **Fix**: edit one file sequentially; parallelize only across different files.
  Re-read the file after a hook may have reformatted it.

### Empty page on `127.0.0.1:7788`

- **Symptom**: Cursor's embedded browser rendered an empty document.
- **Fix**: use `http://localhost:7788`; both are allowed by the Host guard.
