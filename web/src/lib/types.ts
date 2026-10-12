// Types mirror the JSON returned by the Go API (internal/server).

export type Status = "active" | "waiting" | "blocked" | "paused" | "done";
export type Level = "ok" | "info" | "warn" | "crit";
export type Severity = "critical" | "high" | "medium" | "low" | "info";
export type Verdict = "ok" | "attention" | "critical" | "unknown" | "";
export type JobStatus = "queued" | "running" | "done" | "failed" | "canceled";

export interface Project {
  slug: string;
  name: string;
  path: string;
  status: Status;
  summary: string;
  tags: string[];
  reviewer: string;
  schedule: Record<string, string>;
  doc_globs: string[];
  created_at: string;
  updated_at: string;
}

export interface Flag {
  level: Level;
  code: string;
  message: string;
}

export interface Entry {
  id: string;
  at: string;
  kind: "park" | "note" | "command" | "system";
  text?: string;
  stopped?: string;
  next?: string;
  open?: string;
  head?: string;
}

export interface Checklist {
  done: number;
  total: number;
}

export interface Card extends Project {
  health_level: Level;
  health: Flag[];
  intent_filled: boolean;
  /** intent.md still carries the agent's "> **Draft**" line. */
  intent_draft: boolean;
  /** The first commit predates registration: an intent draft is offered. */
  established: boolean;
  branch?: string;
  last_commit_at?: string;
  last_commit_subject?: string;
  dirty_files: number;
  commits_7d: number;
  checklist: Checklist;
  xteam_open: number;
  xteam_closed: number;
  changelog_version?: string;
  doc_count: number;
  stack: string[];
  last_park?: Entry;
  last_scan_at?: string;
  active_jobs: number;
  verdicts: Record<string, Verdict>;
}

export interface Commit {
  hash: string;
  short: string;
  at: string;
  author: string;
  subject: string;
}

export interface Doc {
  path: string;
  kind: string;
  title: string;
  size: number;
  mod_time: string;
  hash: string;
  checked: number;
  unchecked: number;
}

export interface XteamTask {
  slug: string;
  title: string;
  stage: string;
  closed: boolean;
  verdict?: string;
  updated_at: string;
}

export interface GitInfo {
  branch: string;
  head: string;
  last_commit_at?: string;
  dirty_files: number;
  dirty_sample: string[];
  commits_7d: number;
  commits_30d: number;
  total_commits: number;
  first_commit_at?: string;
  remote?: string;
  recent: Commit[];
}

export interface Snapshot {
  scanned_at: string;
  duration_ms: number;
  path_exists: boolean;
  git?: GitInfo;
  docs: Doc[];
  docs_truncated: boolean;
  xteam?: { open: number; closed: number; tasks: XteamTask[] };
  checklist: Checklist;
  changelog?: { path: string; version: string; date?: string; excerpt: string };
  stack: string[];
  errors?: string[];
}

export interface Finding {
  severity: Severity;
  title: string;
  file?: string;
  detail?: string;
  suggestion?: string;
}

export interface Job {
  id: string;
  project: string;
  kind: string;
  question?: string;
  reviewer: string;
  trigger: string;
  status: JobStatus;
  created_at: string;
  started_at?: string;
  finished_at?: string;
  exit_code: number;
  error?: string;
  verdict?: Verdict;
  score?: number;
  summary?: string;
  findings: Finding[];
  tree_changed: boolean;
  prompt_chars: number;
  /** Intent drafts only: the draft filled an empty intent.md. */
  intent_written?: boolean;
}

export interface ADR {
  id: string;
  number: number;
  title: string;
  status: string;
  content: string;
  updated_at: string;
}

export interface Brief {
  card: Card;
  intent: string;
  adrs: ADR[];
  last_visit?: { at: string; head: string };
  since: {
    reference: string;
    at?: string;
    head?: string;
    commits: Commit[];
    docs: Doc[];
    slices: XteamTask[];
    reviews: Job[];
    other_files: number;
  };
  snapshot?: Snapshot;
  latest_reviews: Record<string, Job>;
  active_reviews: Job[];
}

export interface ReviewKind {
  id: string;
  title: string;
  description: string;
  /** Produces a verdict and findings (false for ask and intent drafts). */
  review: boolean;
}

export interface Intent {
  content: string;
  filled: boolean;
  draft: boolean;
}

export interface DraftOutcome {
  project: string;
  code: "queued" | "filled" | "new" | "running" | "error";
  job_id?: string;
  reason?: string;
}

export interface ReviewerInfo {
  name: string;
  description: string;
  command: string[];
  installed: boolean;
}

export interface ConsoleCommand {
  name: string;
  args: string;
  summary: string;
}

export interface Meta {
  version: string;
  home: string;
  statuses: Status[];
  scheduled_kinds: string[];
  schedule_values: string[];
  review_kinds: ReviewKind[];
  doc_kinds: string[];
  reviewers: ReviewerInfo[];
  default_reviewer: string;
  report_language: string;
  commands: ConsoleCommand[];
}

export interface Config {
  listen: string;
  scan_interval: string;
  review_timeout: string;
  max_concurrent_reviews: number;
  default_reviewer: string;
  report_language: string;
  default_schedule: Record<string, string>;
  reviewers: Record<string, { description: string; command: string[] }>;
}

export interface ConsoleResult {
  ok: boolean;
  output: string;
  job_id?: string;
  job_ids?: string[];
  refresh?: string[];
}

export interface HistoryItem {
  at: string;
  input: string;
  result: ConsoleResult;
}

export interface InboxItem {
  project: string;
  project_name: string;
  kind: string;
  job_id: string;
  verdict: Verdict;
  at: string;
  finding: Finding;
}

export interface TreeEntry {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
  sensitive: boolean;
}
