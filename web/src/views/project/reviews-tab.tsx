import { useEffect, useMemo, useRef, useState } from "react";
import {
  ChevronRightIcon,
  CircleStopIcon,
  FileSearchIcon,
  Loader2Icon,
  SendIcon,
  TargetIcon,
  TriangleAlertIcon,
} from "lucide-react";
import { toast } from "sonner";

import { Empty, ErrorBlock, LoadingBlock, Stat } from "@/components/common";
import { Markdown } from "@/components/markdown";
import {
  JobStatusBadge,
  KindIcon,
  SeverityBadge,
  VerdictBadge,
} from "@/components/status";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tip } from "@/components/ui/tooltip";
import { api } from "@/lib/api";
import {
  dateTime,
  duration,
  jobActive,
  jobTitle,
  kindLabel,
  timeAgo,
} from "@/lib/format";
import { t, tn } from "@/lib/i18n";
import { projectFileResolver } from "@/lib/links";
import { invalidate, useQuery } from "@/lib/query";
import { jobReviewer } from "@/lib/reviewers";
import { navigate, routeHref } from "@/lib/route";
import type { Finding, Job, Severity } from "@/lib/types";
import { refreshProject, useMeta } from "@/lib/ui-state";
import { cn } from "@/lib/utils";

const severityOrder: Severity[] = ["critical", "high", "medium", "low", "info"];
const reportLabel: Record<string, string> = { ask: "Answer", intent: "Draft" };
const severityCount: Record<Severity, string> = {
  critical: "{count} critical",
  high: "{count} high",
  medium: "{count} medium",
  low: "{count} low",
  info: "{count} info",
};

/** Read-only external reviews: run, watch, and read the findings. */
export function ReviewsTab({
  slug,
  arg,
  projectPath,
}: {
  slug: string;
  arg?: string;
  projectPath: string;
}) {
  const [anyActive, setAnyActive] = useState(false);
  const reviews = useQuery(`reviews:${slug}`, () => api.reviews(slug), {
    refetchInterval: anyActive ? 3000 : 20000,
  });
  const jobs = reviews.data?.jobs ?? [];
  const active = jobs.some((job) => jobActive(job.status));
  if (active !== anyActive) setAnyActive(active);
  const selectedId = arg ?? jobs[0]?.id;

  if (reviews.error && !reviews.data)
    return <ErrorBlock error={reviews.error} />;
  if (!reviews.data) return <LoadingBlock />;

  return (
    <div className="flex h-full min-h-0">
      <aside className="flex w-80 shrink-0 flex-col border-r">
        <RunBar slug={slug} />
        <div className="min-h-0 flex-1 overflow-y-auto py-1">
          {jobs.length === 0 && (
            <p className="p-3 text-xs text-muted-foreground">
              {t(
                "No reviews yet. Start with Drift: it checks whether the code still serves the intent.",
              )}
            </p>
          )}
          {jobs.map((job) => (
            <JobRow
              key={job.id}
              slug={slug}
              job={job}
              active={job.id === selectedId}
            />
          ))}
        </div>
      </aside>
      <div className="min-w-0 flex-1">
        {selectedId ? (
          <JobDetail
            key={selectedId}
            slug={slug}
            id={selectedId}
            projectPath={projectPath}
          />
        ) : (
          <Empty
            icon={<FileSearchIcon />}
            title={t("No review selected")}
            className="h-full"
          >
            {t(
              "Reviews run an agent in read-only mode against the project, with xplan's intent, decisions and progress as context.",
            )}
          </Empty>
        )}
      </div>
    </div>
  );
}

function RunBar({ slug }: { slug: string }) {
  const meta = useMeta();
  const [question, setQuestion] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const kinds = (meta.data?.review_kinds ?? []).filter((kind) => kind.review);

  const start = async (kind: string, text = "") => {
    setBusy(kind);
    try {
      const job = await api.startReview(slug, kind, text);
      toast.success(
        kind === "ask"
          ? t("Asked {reviewer}", { reviewer: jobReviewer(job) })
          : t("{kind} review queued with {reviewer}", {
              kind: kindLabel(kind),
              reviewer: jobReviewer(job),
            }),
      );
      if (kind === "ask") setQuestion("");
      refreshProject(slug);
      invalidate("overview");
      navigate({ view: "project", slug, tab: "reviews", arg: job.id });
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="space-y-2 border-b p-2">
      <div className="flex flex-wrap gap-1">
        {kinds.map((kind) => (
          <Tip key={kind.id} label={kind.description}>
            <Button
              variant="outline"
              size="xs"
              disabled={busy !== null}
              onClick={() => void start(kind.id)}
            >
              {busy === kind.id ? (
                <Loader2Icon className="animate-spin" />
              ) : (
                <KindIcon kind={kind.id} />
              )}
              <span className="capitalize">{kindLabel(kind.id)}</span>
            </Button>
          </Tip>
        ))}
      </div>
      <form
        className="flex gap-1"
        onSubmit={(event) => {
          event.preventDefault();
          if (question.trim()) void start("ask", question.trim());
        }}
      >
        <Input
          value={question}
          onChange={(event) => setQuestion(event.target.value)}
          placeholder={t("Ask the reviewer about this project…")}
          className="h-7 text-[12.5px]"
        />
        <Button
          type="submit"
          size="icon-sm"
          variant="secondary"
          disabled={!question.trim() || busy !== null}
        >
          {busy === "ask" ? (
            <Loader2Icon className="animate-spin" />
          ) : (
            <SendIcon />
          )}
        </Button>
      </form>
    </div>
  );
}

function JobRow({
  slug,
  job,
  active,
}: {
  slug: string;
  job: Job;
  active: boolean;
}) {
  return (
    <a
      href={routeHref({ view: "project", slug, tab: "reviews", arg: job.id })}
      className={cn(
        "mx-1 flex flex-col gap-0.5 rounded-md px-2 py-1.5 hover:bg-accent/50",
        active && "bg-accent",
      )}
    >
      <span className="flex items-center gap-2">
        <KindIcon kind={job.kind} className="text-muted-foreground" />
        <span className="text-[12.5px] capitalize">{kindLabel(job.kind)}</span>
        <span className="ml-auto">
          {job.status === "done" ? (
            <VerdictBadge verdict={job.verdict} kind={job.kind} />
          ) : (
            <JobStatusBadge status={job.status} />
          )}
        </span>
      </span>
      <span className="truncate pl-5.5 text-[11px] text-muted-foreground">
        {job.kind === "ask"
          ? job.question
          : job.summary ||
            tn(job.findings.length, "{count} finding", "{count} findings")}
      </span>
      <span className="pl-5.5 text-[10.5px] text-muted-foreground/70">
        {timeAgo(job.created_at)} · {t(job.trigger)} · {jobReviewer(job)}
      </span>
    </a>
  );
}

function JobDetail({
  slug,
  id,
  projectPath,
}: {
  slug: string;
  id: string;
  projectPath: string;
}) {
  const [polling, setPolling] = useState(true);
  const resolveHref = useMemo(
    () => projectFileResolver(slug, projectPath),
    [slug, projectPath],
  );
  const detail = useQuery(`review:${slug}:${id}`, () => api.review(slug, id), {
    refetchInterval: polling ? 2500 : false,
  });
  const job = detail.data?.job;
  const finished = job !== undefined && !jobActive(job.status);
  const sawRunning = useRef(false);
  if (job && jobActive(job.status)) sawRunning.current = true;
  useEffect(() => {
    if (!finished || !polling) return;
    setPolling(false);
    if (sawRunning.current) refreshProject(slug);
  }, [finished, polling, slug]);

  const cancel = async () => {
    try {
      await api.cancelReview(slug, id);
      toast.success(t("Cancel requested"));
      invalidate(`review:${slug}:${id}`, `reviews:${slug}`);
    } catch (error) {
      toast.error((error as Error).message);
    }
  };

  if (detail.error && !detail.data) return <ErrorBlock error={detail.error} />;
  if (!job) return <LoadingBlock />;

  const running = jobActive(job.status);
  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-4xl space-y-4 p-5">
        <header className="flex flex-wrap items-center gap-2">
          <KindIcon kind={job.kind} className="size-4 text-muted-foreground" />
          <h2 className="text-[15px] font-semibold capitalize">
            {jobTitle(job.kind)}
          </h2>
          {job.status === "done" ? (
            <VerdictBadge verdict={job.verdict} kind={job.kind} />
          ) : (
            <JobStatusBadge status={job.status} />
          )}
          {typeof job.score === "number" && job.score > 0 && (
            <Badge variant="outline">
              {t("score {score}/100", { score: job.score })}
            </Badge>
          )}
          {running && (
            <Button
              variant="destructive"
              size="xs"
              className="ml-auto"
              onClick={() => void cancel()}
            >
              <CircleStopIcon />
              {t("Cancel")}
            </Button>
          )}
          {!running && job.kind === "intent" && job.status === "done" && (
            <a
              href={routeHref({ view: "project", slug, tab: "intent" })}
              className={cn(
                buttonVariants({ variant: "outline", size: "xs" }),
                "ml-auto",
              )}
            >
              <TargetIcon />
              {t("Open the Intent tab")}
            </a>
          )}
        </header>

        <div className="grid grid-cols-2 gap-3 rounded-lg border bg-card/60 p-3 sm:grid-cols-4">
          <Stat label={t("Reviewer")} value={jobReviewer(job)} />
          <Stat
            label={t("Started")}
            value={dateTime(job.started_at ?? job.created_at)}
          />
          <Stat
            label={t("Duration")}
            value={duration(job.started_at ?? job.created_at, job.finished_at)}
          />
          <Stat
            label={t("Trigger")}
            value={t(job.trigger)}
            hint={tn(
              job.prompt_chars,
              "{count} prompt character",
              "{count} prompt characters",
              { count: job.prompt_chars.toLocaleString() },
            )}
          />
        </div>

        {job.attempts && job.attempts.length > 0 && (
          <div className="rounded-lg border bg-card/60 p-3">
            <div className="mb-1 text-[11px] text-muted-foreground">
              {t("Skipped by auto before picking a reviewer")}
            </div>
            <ul className="space-y-0.5 font-mono text-[11.5px] text-muted-foreground">
              {job.attempts.map((attempt) => (
                <li key={attempt}>{attempt}</li>
              ))}
            </ul>
          </div>
        )}

        {job.question && (
          <div className="rounded-lg border bg-card/60 p-3 text-[13px]">
            <div className="mb-1 text-[11px] text-muted-foreground">
              {t("You asked")}
            </div>
            {job.question}
          </div>
        )}

        {running && (
          <div className="flex items-center gap-2 rounded-lg border border-info/30 bg-info/5 p-3 text-[13px] text-info">
            <Loader2Icon className="size-4 animate-spin" />
            {job.status === "queued"
              ? t("Waiting for a free review slot…")
              : t(
                  "{reviewer} is reading the project (read-only). This usually takes a few minutes.",
                  { reviewer: jobReviewer(job) },
                )}
          </div>
        )}

        {job.error && (
          <div className="rounded-lg border border-crit/30 bg-crit/5 p-3 font-mono text-[12px] whitespace-pre-wrap text-crit">
            {job.error}
          </div>
        )}

        {job.tree_changed && (
          <div className="flex items-start gap-2 rounded-lg border border-warn/30 bg-warn/5 p-3 text-[12.5px] text-warn">
            <TriangleAlertIcon className="mt-0.5 size-4 shrink-0" />
            {t(
              "The project's working tree changed while this review ran. That may be you editing in parallel — or the agent writing despite read-only mode. Check git status.",
            )}
          </div>
        )}

        {job.summary && job.kind !== "ask" && (
          <div className="rounded-lg border bg-card/60 p-3">
            <div className="mb-1 text-[11px] text-muted-foreground">
              {t("Summary")}
            </div>
            <p className="text-[13px]">{job.summary}</p>
          </div>
        )}

        {job.findings.length > 0 && <FindingsTable findings={job.findings} />}

        {!running && detail.data?.report && (
          <details
            open={job.kind === "ask" || job.findings.length === 0}
            className="group rounded-lg border bg-card/60"
          >
            <summary className="flex cursor-pointer items-center gap-2 px-3 py-2 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase select-none">
              <ChevronRightIcon className="size-3 transition-transform group-open:rotate-90" />
              {t(reportLabel[job.kind] ?? "Full report")}
            </summary>
            <div className="border-t p-4">
              <Markdown resolveHref={resolveHref}>
                {detail.data.report}
              </Markdown>
            </div>
          </details>
        )}
      </div>
    </div>
  );
}

function FindingsTable({ findings }: { findings: Finding[] }) {
  const sorted = [...findings].sort(
    (a, b) =>
      severityOrder.indexOf(a.severity) - severityOrder.indexOf(b.severity),
  );
  const counts = severityOrder
    .map(
      (severity) =>
        [
          severity,
          findings.filter((finding) => finding.severity === severity).length,
        ] as const,
    )
    .filter(([, count]) => count > 0);
  return (
    <section className="rounded-lg border bg-card/60">
      <header className="flex items-center gap-2 border-b px-3 py-2">
        <span className="text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">
          {t("Findings · {count}", { count: findings.length })}
        </span>
        <span className="ml-auto flex gap-2 text-[11px] text-muted-foreground">
          {counts.map(([severity, count]) => (
            <span key={severity}>{t(severityCount[severity], { count })}</span>
          ))}
        </span>
      </header>
      <ul className="divide-y">
        {sorted.map((finding, index) => (
          <FindingRow key={index} finding={finding} />
        ))}
      </ul>
    </section>
  );
}

function FindingRow({ finding }: { finding: Finding }) {
  const [open, setOpen] = useState(
    finding.severity === "critical" || finding.severity === "high",
  );
  const expandable = Boolean(finding.detail || finding.suggestion);
  return (
    <li>
      <button
        type="button"
        onClick={() => expandable && setOpen(!open)}
        className="flex w-full items-center gap-3 px-3 py-2 text-left hover:bg-accent/30"
      >
        <ChevronRightIcon
          className={cn(
            "size-3.5 shrink-0 text-muted-foreground transition-transform",
            open && "rotate-90",
            !expandable && "invisible",
          )}
        />
        <SeverityBadge severity={finding.severity} />
        <span className="min-w-0 flex-1 text-[13px]">{finding.title}</span>
        {finding.file && (
          <code className="max-w-72 shrink-0 truncate font-mono text-[11px] text-muted-foreground">
            {finding.file}
          </code>
        )}
      </button>
      {open && expandable && (
        <div className="space-y-2 px-3 pb-3 pl-[7.25rem] text-[12.5px]">
          {finding.detail && (
            <p className="whitespace-pre-wrap">{finding.detail}</p>
          )}
          {finding.suggestion && (
            <p className="whitespace-pre-wrap text-muted-foreground">
              <span className="font-medium text-foreground">
                {t("Suggestion:")}{" "}
              </span>
              {finding.suggestion}
            </p>
          )}
        </div>
      )}
    </li>
  );
}
