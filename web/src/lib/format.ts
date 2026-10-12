import type { JobStatus, Level, Severity, Status, Verdict } from "@/lib/types";

/** "3m", "5h", "12d" style relative age. */
export function ago(iso?: string | null): string {
  if (!iso) return "—";
  const seconds = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (seconds < 60) return "now";
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
  if (seconds < 172800) return `${Math.floor(seconds / 3600)}h`;
  if (seconds < 86400 * 60) return `${Math.floor(seconds / 86400)}d`;
  return `${Math.floor(seconds / (86400 * 30))}mo`;
}

export function dateTime(iso?: string | null): string {
  if (!iso) return "—";
  const date = new Date(iso);
  return date.toLocaleString(undefined, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function duration(startIso?: string, endIso?: string): string {
  if (!startIso) return "—";
  const ms =
    (endIso ? new Date(endIso).getTime() : Date.now()) -
    new Date(startIso).getTime();
  const seconds = Math.round(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
}

export function percent(done: number, total: number): number {
  return total > 0 ? Math.round((done / total) * 100) : 0;
}

export const levelDot: Record<Level, string> = {
  ok: "bg-ok",
  info: "bg-info",
  warn: "bg-warn",
  crit: "bg-crit",
};

export const levelText: Record<Level, string> = {
  ok: "text-ok",
  info: "text-info",
  warn: "text-warn",
  crit: "text-crit",
};

export function verdictVariant(
  verdict?: Verdict,
): "ok" | "warn" | "crit" | "muted" {
  if (verdict === "ok") return "ok";
  if (verdict === "attention") return "warn";
  if (verdict === "critical") return "crit";
  return "muted";
}

export function verdictLabel(
  verdict: Verdict | undefined,
  kind?: string,
): string {
  if (kind === "drift") {
    if (verdict === "ok") return "aligned";
    if (verdict === "attention") return "drifting";
    if (verdict === "critical") return "off-track";
  }
  if (!verdict && kind === "intent") return "draft";
  if (!verdict && kind === "ask") return "answered";
  if (!verdict) return "—";
  return verdict;
}

/** Heading for a job: "Question", "Intent draft" or "<kind> review". */
export function jobTitle(kind: string): string {
  if (kind === "ask") return "Question";
  if (kind === "intent") return "Intent draft";
  return `${kind} review`;
}

export const severityVariant: Record<
  Severity,
  "crit" | "warn" | "info" | "muted"
> = {
  critical: "crit",
  high: "crit",
  medium: "warn",
  low: "info",
  info: "muted",
};

export const statusLabel: Record<Status, string> = {
  active: "Active",
  waiting: "Waiting",
  blocked: "Blocked",
  paused: "Paused",
  done: "Done",
};

export const statusHint: Record<Status, string> = {
  active: "You are working on it",
  waiting: "Waiting on an agent, a review or someone else",
  blocked: "Cannot move until something is resolved",
  paused: "Deliberately parked for later",
  done: "Shipped or finished",
};

export const statusColor: Record<Status, string> = {
  active: "bg-primary",
  waiting: "bg-info",
  blocked: "bg-crit",
  paused: "bg-muted-foreground",
  done: "bg-ok",
};

export function jobActive(status: JobStatus) {
  return status === "queued" || status === "running";
}

export function fileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
