# Error Diary

Recurring problems and how they were resolved. Newest first.

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
- **Status**: open. Use `codex` or `cursor-agent` meanwhile.

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
