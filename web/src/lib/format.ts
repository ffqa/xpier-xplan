import { getLanguage, t } from "@/lib/i18n";
import type { JobStatus, Level, Severity, Status, Verdict } from "@/lib/types";

type AgeUnit = "now" | "m" | "h" | "d" | "mo";

function age(iso: string): { unit: AgeUnit; n: number } {
  const seconds = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (seconds < 60) return { unit: "now", n: 0 };
  if (seconds < 3600) return { unit: "m", n: Math.floor(seconds / 60) };
  if (seconds < 172800) return { unit: "h", n: Math.floor(seconds / 3600) };
  if (seconds < 86400 * 60)
    return { unit: "d", n: Math.floor(seconds / 86400) };
  return { unit: "mo", n: Math.floor(seconds / (86400 * 30)) };
}

/** "3m", "5h", "12d" style relative age. */
export function ago(iso?: string | null): string {
  if (!iso) return "—";
  const { unit, n } = age(iso);
  switch (unit) {
    case "now":
      return t("now");
    case "m":
      return t("{n}m", { n });
    case "h":
      return t("{n}h", { n });
    case "d":
      return t("{n}d", { n });
    case "mo":
      return t("{n}mo", { n });
  }
}

/** Full relative phrase for running text: "3m ago", "just now". */
export function timeAgo(iso?: string | null): string {
  if (!iso) return "—";
  const { unit, n } = age(iso);
  switch (unit) {
    case "now":
      return t("just now");
    case "m":
      return t("{n}m ago", { n });
    case "h":
      return t("{n}h ago", { n });
    case "d":
      return t("{n}d ago", { n });
    case "mo":
      return t("{n}mo ago", { n });
  }
}

/** Locale for Intl date formatting; undefined keeps the browser default. */
export function dateLocale(): string | undefined {
  return getLanguage() === "zh" ? "zh-CN" : undefined;
}

export function dateTime(iso?: string | null): string {
  if (!iso) return "—";
  const date = new Date(iso);
  return date.toLocaleString(dateLocale(), {
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
  if (seconds < 60) return t("{s}s", { s: seconds });
  return t("{m}m {s}s", { m: Math.floor(seconds / 60), s: seconds % 60 });
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
    if (verdict === "ok") return t("aligned");
    if (verdict === "attention") return t("drifting");
    if (verdict === "critical") return t("off-track");
  }
  if (!verdict && kind === "intent") return t("draft");
  if (!verdict && kind === "ask") return t("answered");
  if (!verdict) return "—";
  switch (verdict) {
    case "ok":
      return t("ok");
    case "attention":
      return t("attention");
    case "critical":
      return t("critical");
    case "unknown":
      return t("unknown");
  }
}

/** Display name for a review/job kind ID; unknown kinds are shown as-is. */
export function kindLabel(kind: string): string {
  switch (kind) {
    case "drift":
      return t("drift");
    case "code":
      return t("code");
    case "security":
      return t("security");
    case "deploy":
      return t("deploy");
    case "test":
      return t("test");
    case "ask":
      return t("ask");
    case "intent":
      return t("intent");
    default:
      return kind;
  }
}

export function jobStatusLabel(status: JobStatus): string {
  switch (status) {
    case "queued":
      return t("queued");
    case "running":
      return t("running");
    case "done":
      return t("done");
    case "failed":
      return t("failed");
    case "canceled":
      return t("canceled");
  }
}

export function severityLabel(severity: Severity): string {
  switch (severity) {
    case "critical":
      return t("critical");
    case "high":
      return t("high");
    case "medium":
      return t("medium");
    case "low":
      return t("low");
    case "info":
      return t("info");
  }
}

/** Heading for a job: "Question", "Intent draft" or "<kind> review". */
export function jobTitle(kind: string): string {
  if (kind === "ask") return t("Question");
  if (kind === "intent") return t("Intent draft");
  switch (kind) {
    case "drift":
      return t("drift review");
    case "code":
      return t("code review");
    case "security":
      return t("security review");
    case "deploy":
      return t("deploy review");
    case "test":
      return t("test review");
    default:
      return t("{kind} review", { kind });
  }
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

/** Record whose English values are translated each time they are read. */
function translatedRecord<K extends string>(
  source: Record<K, string>,
): Record<K, string> {
  const translated = {} as Record<K, string>;
  for (const key of Object.keys(source) as K[]) {
    Object.defineProperty(translated, key, {
      enumerable: true,
      get: () => t(source[key]),
    });
  }
  return translated;
}

export const statusLabel: Record<Status, string> = translatedRecord({
  active: "Active",
  waiting: "Waiting",
  blocked: "Blocked",
  paused: "Paused",
  done: "Done",
});

export const statusHint: Record<Status, string> = translatedRecord({
  active: "You are working on it",
  waiting: "Waiting on an agent, a review or someone else",
  blocked: "Cannot move until something is resolved",
  paused: "Deliberately parked for later",
  done: "Shipped or finished",
});

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
