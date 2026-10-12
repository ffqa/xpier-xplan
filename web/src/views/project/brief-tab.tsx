import { useState } from "react";
import {
  ActivityIcon,
  CheckCheckIcon,
  FileTextIcon,
  GitCommitHorizontalIcon,
  HeartPulseIcon,
  LightbulbIcon,
  Loader2Icon,
  PlayIcon,
  SquareParkingIcon,
  TargetIcon,
  UsersIcon,
  WandSparklesIcon,
} from "lucide-react";
import { toast } from "sonner";

import { Section, Stat } from "@/components/common";
import { Markdown } from "@/components/markdown";
import { FlagList, KindIcon, VerdictBadge } from "@/components/status";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Progress } from "@/components/ui/misc";
import { api } from "@/lib/api";
import {
  ago,
  dateTime,
  duration,
  jobStatusLabel,
  kindLabel,
  percent,
  timeAgo,
} from "@/lib/format";
import { t, tn } from "@/lib/i18n";
import { jobReviewer } from "@/lib/reviewers";
import { BRIEF_SECTIONS, sectionBody } from "@/lib/intent";
import { useIntentDraft } from "@/lib/intent-draft";
import { invalidate } from "@/lib/query";
import { navigate, routeHref } from "@/lib/route";
import type { Brief } from "@/lib/types";
import { refreshProject, useMeta, useUI } from "@/lib/ui-state";

/**
 * The re-entry brief: intent first, then where you stopped, then what moved
 * while you were away. Designed to be read top to bottom in under a minute.
 */
export function BriefTab({ slug, brief }: { slug: string; brief: Brief }) {
  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto grid max-w-6xl gap-4 p-4 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="min-w-0 space-y-4">
          <IntentCard slug={slug} brief={brief} />
          <ParkCard slug={slug} brief={brief} />
          <SinceCard slug={slug} brief={brief} />
        </div>
        <div className="min-w-0 space-y-4">
          <Section title={t("Health")} icon={<HeartPulseIcon />}>
            <FlagList flags={brief.card.health} />
          </Section>
          <ReviewsCard slug={slug} brief={brief} />
          <ProgressCard brief={brief} />
          <DecisionsCard slug={slug} brief={brief} />
        </div>
      </div>
    </div>
  );
}

function IntentCard({ slug, brief }: { slug: string; brief: Brief }) {
  const sections = BRIEF_SECTIONS.map((section) => ({
    ...section,
    body: sectionBody(brief.intent, section.prefix),
  })).filter((section) => section.body);
  const edit = (
    <a
      href={routeHref({ view: "project", slug, tab: "intent" })}
      className={buttonVariants({ variant: "ghost", size: "xs" })}
    >
      {t("Edit")}
    </a>
  );
  if (!brief.card.intent_filled || sections.length === 0) {
    return (
      <Section
        title={t("Why this exists")}
        icon={<TargetIcon />}
        actions={edit}
        className="border-warn/40"
      >
        <p className="text-[13px]">{t("The intent is still empty.")}</p>
        <p className="mt-1 text-xs text-muted-foreground">
          {brief.card.established
            ? t(
                "This project already has history. Let the reviewer reconstruct why it was started, then correct the draft in your own words.",
              )
            : t(
                "Write down why this project exists and what “done” means, in your own words. Drift reviews compare the code against it, and it is the first thing you will read when you come back.",
              )}
        </p>
        <div className="mt-3 flex flex-wrap items-center gap-2">
          {brief.card.established && <DraftIntentButton slug={slug} primary />}
          <Button
            size="sm"
            variant={brief.card.established ? "outline" : "default"}
            onClick={() => navigate({ view: "project", slug, tab: "intent" })}
          >
            {t("Write it myself")}
          </Button>
        </div>
      </Section>
    );
  }
  return (
    <Section
      title={
        <>
          {t("Why this exists")}
          {brief.card.intent_draft && (
            <Badge variant="warn" className="ml-2 normal-case">
              {t("draft — needs your edit")}
            </Badge>
          )}
        </>
      }
      icon={<TargetIcon />}
      actions={edit}
      className={brief.card.intent_draft ? "border-warn/40" : undefined}
    >
      <div className="grid gap-3 sm:grid-cols-2">
        {sections.map((section) => (
          <div key={section.prefix} className="min-w-0">
            <div className="mb-1 text-[11px] font-medium text-muted-foreground">
              {t(section.label)}
            </div>
            <Markdown className="text-[13px]">{section.body}</Markdown>
          </div>
        ))}
      </div>
    </Section>
  );
}

function DraftIntentButton({
  slug,
  primary,
}: {
  slug: string;
  primary?: boolean;
}) {
  const draft = useIntentDraft(slug);
  if (draft.running && draft.job) {
    return (
      <span className="flex items-center gap-1.5 text-xs text-info">
        <Loader2Icon className="size-3.5 animate-spin" />
        {t("{reviewer} is drafting the intent…", {
          reviewer: jobReviewer(draft.job),
        })}
      </span>
    );
  }
  return (
    <Button
      size="sm"
      variant={primary ? "default" : "outline"}
      disabled={draft.starting}
      onClick={() => void draft.start()}
    >
      {draft.starting ? (
        <Loader2Icon className="animate-spin" />
      ) : (
        <WandSparklesIcon />
      )}
      {t("Draft from history")}
    </Button>
  );
}

function ParkCard({ slug, brief }: { slug: string; brief: Brief }) {
  const ui = useUI();
  const park = brief.card.last_park;
  const action = (
    <Button variant="ghost" size="xs" onClick={() => ui.openPark(slug)}>
      <SquareParkingIcon />
      {t("Park now")}
    </Button>
  );
  if (!park) {
    return (
      <Section
        title={t("Where you left off")}
        icon={<SquareParkingIcon />}
        actions={action}
      >
        <p className="text-xs text-muted-foreground">
          {t(
            "No park note yet. Before you switch to another project, press Park and write one line about where you stopped.",
          )}
        </p>
      </Section>
    );
  }
  return (
    <Section
      title={
        <>
          {t("Where you left off")}{" "}
          <span className="font-normal normal-case">· {timeAgo(park.at)}</span>
        </>
      }
      icon={<SquareParkingIcon />}
      actions={action}
    >
      <dl className="grid gap-2 text-[13px]">
        <ParkLine label={t("Stopped at")} value={park.stopped} />
        <ParkLine label={t("Next step")} value={park.next} highlight />
        <ParkLine label={t("Open questions")} value={park.open} />
      </dl>
      {park.head && (
        <p className="mt-2 text-[11px] text-muted-foreground">
          {t("Parked at commit {hash}", { hash: park.head.slice(0, 8) })}
        </p>
      )}
    </Section>
  );
}

function ParkLine({
  label,
  value,
  highlight,
}: {
  label: string;
  value?: string;
  highlight?: boolean;
}) {
  if (!value) return null;
  return (
    <div className="grid grid-cols-[110px_1fr] gap-2">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className={highlight ? "font-medium text-primary" : ""}>{value}</dd>
    </div>
  );
}

function SinceCard({ slug, brief }: { slug: string; brief: Brief }) {
  const [busy, setBusy] = useState(false);
  const { since } = brief;
  const nothing =
    since.commits.length === 0 &&
    since.docs.length === 0 &&
    since.slices.length === 0 &&
    since.reviews.length === 0 &&
    since.other_files === 0;

  const caughtUp = async () => {
    setBusy(true);
    try {
      await api.visit(slug);
      toast.success(t("Marked as caught up"));
      refreshProject(slug);
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const title = since.reference ? (
    <>
      {sinceTitle(since.reference)}{" "}
      <span className="font-normal normal-case">· {timeAgo(since.at)}</span>
    </>
  ) : (
    t("Recent activity")
  );

  return (
    <Section
      title={title}
      icon={<ActivityIcon />}
      actions={
        <Button
          variant="ghost"
          size="xs"
          onClick={() => void caughtUp()}
          disabled={busy}
        >
          {busy ? <Loader2Icon className="animate-spin" /> : <CheckCheckIcon />}
          {t("I'm caught up")}
        </Button>
      }
    >
      {!since.reference && <RecentCommits brief={brief} />}
      {since.reference && nothing && (
        <p className="text-xs text-muted-foreground">
          {t("Nothing changed since then.")}
        </p>
      )}
      {since.reference && !nothing && (
        <div className="space-y-4">
          {since.commits.length > 0 && (
            <SinceGroup
              title={tn(
                since.commits.length,
                "{count} commit",
                "{count} commits",
              )}
            >
              <ul className="space-y-1">
                {since.commits.map((commit) => (
                  <li
                    key={commit.hash}
                    className="flex items-baseline gap-2 text-[12.5px]"
                  >
                    <GitCommitHorizontalIcon className="size-3 shrink-0 self-center text-muted-foreground" />
                    <code className="font-mono text-[11px] text-muted-foreground">
                      {commit.short}
                    </code>
                    <span className="min-w-0 flex-1 truncate">
                      {commit.subject}
                    </span>
                    <span className="shrink-0 text-[11px] text-muted-foreground">
                      {ago(commit.at)}
                    </span>
                  </li>
                ))}
              </ul>
              {since.other_files > 0 && (
                <p className="mt-1 text-[11px] text-muted-foreground">
                  {tn(
                    since.other_files,
                    "{count} non-doc file changed.",
                    "{count} non-doc files changed.",
                  )}
                </p>
              )}
            </SinceGroup>
          )}
          {since.docs.length > 0 && (
            <SinceGroup
              title={tn(
                since.docs.length,
                "{count} doc changed",
                "{count} docs changed",
              )}
            >
              <ul className="space-y-1">
                {since.docs.map((doc) => (
                  <li key={doc.path}>
                    <a
                      href={routeHref({
                        view: "project",
                        slug,
                        tab: "docs",
                        arg: doc.path,
                      })}
                      className="flex items-center gap-2 text-[12.5px] hover:text-primary"
                    >
                      <FileTextIcon className="size-3 shrink-0 text-muted-foreground" />
                      <span className="truncate">{doc.title || doc.path}</span>
                      <Badge variant="muted">{doc.kind}</Badge>
                      <span className="ml-auto truncate font-mono text-[11px] text-muted-foreground">
                        {doc.path}
                      </span>
                    </a>
                  </li>
                ))}
              </ul>
            </SinceGroup>
          )}
          {since.slices.length > 0 && (
            <SinceGroup
              title={tn(
                since.slices.length,
                "{count} xteam slice moved",
                "{count} xteam slices moved",
              )}
            >
              <ul className="space-y-1">
                {since.slices.map((task) => (
                  <li
                    key={task.slug}
                    className="flex items-center gap-2 text-[12.5px]"
                  >
                    <UsersIcon className="size-3 shrink-0 text-muted-foreground" />
                    <span className="truncate">{task.title || task.slug}</span>
                    <Badge variant={task.closed ? "ok" : "info"}>
                      {task.closed ? t("closed") : task.stage}
                    </Badge>
                    {task.verdict && (
                      <Badge variant="muted">{task.verdict}</Badge>
                    )}
                  </li>
                ))}
              </ul>
            </SinceGroup>
          )}
          {since.reviews.length > 0 && (
            <SinceGroup
              title={tn(
                since.reviews.length,
                "{count} review finished",
                "{count} reviews finished",
              )}
            >
              <ul className="space-y-1">
                {since.reviews.map((job) => (
                  <li key={job.id}>
                    <a
                      href={routeHref({
                        view: "project",
                        slug,
                        tab: "reviews",
                        arg: job.id,
                      })}
                      className="flex items-center gap-2 text-[12.5px] hover:text-primary"
                    >
                      <KindIcon
                        kind={job.kind}
                        className="text-muted-foreground"
                      />
                      <VerdictBadge verdict={job.verdict} kind={job.kind} />
                      <span className="truncate">
                        {job.summary || job.kind}
                      </span>
                    </a>
                  </li>
                ))}
              </ul>
            </SinceGroup>
          )}
        </div>
      )}
    </Section>
  );
}

/** The server sends "last visit" or "last park note" as the reference. */
function sinceTitle(reference: string): string {
  if (reference === "last visit") return t("Since your last visit");
  if (reference === "last park note") return t("Since your last park note");
  return t("Since your {reference}", { reference });
}

function RecentCommits({ brief }: { brief: Brief }) {
  const recent = brief.snapshot?.git?.recent ?? [];
  if (recent.length === 0)
    return (
      <p className="text-xs text-muted-foreground">
        {t(
          "No commits found. Park this project once and xplan will show what changed from then on.",
        )}
      </p>
    );
  return (
    <>
      <p className="mb-2 text-xs text-muted-foreground">
        {t(
          "First visit — showing the latest commits. From now on this card shows only what changed since you last looked.",
        )}
      </p>
      <ul className="space-y-1">
        {recent.slice(0, 8).map((commit) => (
          <li
            key={commit.hash}
            className="flex items-baseline gap-2 text-[12.5px]"
          >
            <code className="font-mono text-[11px] text-muted-foreground">
              {commit.short}
            </code>
            <span className="min-w-0 flex-1 truncate">{commit.subject}</span>
            <span className="shrink-0 text-[11px] text-muted-foreground">
              {ago(commit.at)}
            </span>
          </li>
        ))}
      </ul>
    </>
  );
}

function SinceGroup({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <div className="mb-1.5 text-[11px] font-medium text-muted-foreground">
        {title}
      </div>
      {children}
    </div>
  );
}

function ReviewsCard({ slug, brief }: { slug: string; brief: Brief }) {
  const meta = useMeta();
  const kinds = (meta.data?.review_kinds ?? []).filter((kind) => kind.review);
  const activeKinds = new Set(brief.active_reviews.map((job) => job.kind));

  const run = async (kind: string) => {
    try {
      await api.startReview(slug, kind);
      toast.success(t("{kind} review queued", { kind }));
      refreshProject(slug);
      invalidate("overview");
    } catch (error) {
      toast.error((error as Error).message);
    }
  };

  return (
    <Section
      title={t("External reviews")}
      icon={<FileTextIcon />}
      bodyClassName="p-1"
    >
      <ul>
        {kinds.map((kind) => {
          const job = brief.latest_reviews[kind.id];
          const running = activeKinds.has(kind.id);
          return (
            <li
              key={kind.id}
              className="group flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-accent/40"
            >
              <KindIcon kind={kind.id} className="text-muted-foreground" />
              <a
                href={
                  job
                    ? routeHref({
                        view: "project",
                        slug,
                        tab: "reviews",
                        arg: job.id,
                      })
                    : undefined
                }
                className="min-w-0 flex-1"
                title={job?.summary}
              >
                <div className="text-[12.5px] capitalize">
                  {kindLabel(kind.id)}
                </div>
                <div className="truncate text-[11px] text-muted-foreground">
                  {job
                    ? `${timeAgo(job.created_at)} · ${tn(
                        job.findings.length,
                        "{count} finding",
                        "{count} findings",
                      )}`
                    : t("never run")}
                </div>
              </a>
              {running ? (
                <Badge variant="info">
                  <Loader2Icon className="animate-spin" />
                  {jobStatusLabel("running")}
                </Badge>
              ) : (
                <>
                  {job && <VerdictBadge verdict={job.verdict} kind={kind.id} />}
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    className="opacity-0 group-hover:opacity-100"
                    title={t("Run {title}", { title: kind.title })}
                    onClick={() => void run(kind.id)}
                  >
                    <PlayIcon />
                  </Button>
                </>
              )}
            </li>
          );
        })}
      </ul>
      {brief.active_reviews.length > 0 && (
        <p className="px-2 pt-1 pb-1.5 text-[11px] text-muted-foreground">
          {brief.active_reviews
            .map(
              (job) =>
                `${kindLabel(job.kind)} ${jobStatusLabel(job.status)} ${duration(job.started_at ?? job.created_at)}`,
            )
            .join(" · ")}
        </p>
      )}
    </Section>
  );
}

function ProgressCard({ brief }: { brief: Brief }) {
  const { card, snapshot } = brief;
  const checklist = card.checklist ?? { done: 0, total: 0 };
  const git = snapshot?.git;
  return (
    <Section title={t("Progress")} icon={<ActivityIcon />}>
      {checklist.total > 0 && (
        <div className="mb-3">
          <div className="mb-1 flex justify-between text-[11px] text-muted-foreground">
            <span>{t("Checklists across docs")}</span>
            <span className="tabular-nums">
              {checklist.done}/{checklist.total}
            </span>
          </div>
          <Progress value={percent(checklist.done, checklist.total)} />
        </div>
      )}
      <div className="grid grid-cols-2 gap-3">
        <Stat label={t("Branch")} value={card.branch || "—"} />
        <Stat
          label={t("Uncommitted")}
          value={
            git ? tn(git.dirty_files, "{count} file", "{count} files") : "—"
          }
          hint={git?.dirty_sample?.join("\n")}
        />
        <Stat
          label={t("Commits 7d / 30d")}
          value={git ? `${git.commits_7d} / ${git.commits_30d}` : "—"}
        />
        <Stat
          label={t("Last commit")}
          value={card.last_commit_at ? timeAgo(card.last_commit_at) : "—"}
          hint={card.last_commit_subject}
        />
        {(card.xteam_open > 0 || card.xteam_closed > 0) && (
          <Stat
            label={t("xteam slices")}
            value={t("{open} open · {closed} closed", {
              open: card.xteam_open,
              closed: card.xteam_closed,
            })}
          />
        )}
        {card.changelog_version && (
          <Stat
            label={t("Changelog")}
            value={card.changelog_version}
            hint={snapshot?.changelog?.date}
          />
        )}
        <Stat label={t("Docs tracked")} value={card.doc_count} />
        <Stat
          label={t("Scanned")}
          value={card.last_scan_at ? timeAgo(card.last_scan_at) : t("never")}
          hint={dateTime(card.last_scan_at)}
        />
      </div>
    </Section>
  );
}

function DecisionsCard({ slug, brief }: { slug: string; brief: Brief }) {
  const decisionDocs = (brief.snapshot?.docs ?? []).filter(
    (doc) => doc.kind === "adr" || doc.kind === "spec",
  );
  if (brief.adrs.length === 0 && decisionDocs.length === 0) return null;
  return (
    <Section
      title={t("Decisions & specs")}
      icon={<LightbulbIcon />}
      bodyClassName="p-1"
    >
      <ul>
        {brief.adrs.slice(0, 6).map((adr) => (
          <li key={adr.id}>
            <a
              href={routeHref({
                view: "project",
                slug,
                tab: "decisions",
                arg: adr.id,
              })}
              className="flex items-center gap-2 rounded-md px-2 py-1 text-[12.5px] hover:bg-accent/40"
            >
              <span className="font-mono text-[11px] text-muted-foreground">
                {String(adr.number).padStart(4, "0")}
              </span>
              <span className="min-w-0 flex-1 truncate">{adr.title}</span>
              <Badge
                variant={
                  adr.status === "accepted"
                    ? "ok"
                    : adr.status === "superseded"
                      ? "muted"
                      : "info"
                }
              >
                {adr.status}
              </Badge>
            </a>
          </li>
        ))}
        {decisionDocs.slice(0, 8).map((doc) => (
          <li key={doc.path}>
            <a
              href={routeHref({
                view: "project",
                slug,
                tab: "docs",
                arg: doc.path,
              })}
              className="flex items-center gap-2 rounded-md px-2 py-1 text-[12.5px] hover:bg-accent/40"
            >
              <FileTextIcon className="size-3 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1 truncate">
                {doc.title || doc.path}
              </span>
              <Badge variant="muted">{doc.kind}</Badge>
            </a>
          </li>
        ))}
      </ul>
    </Section>
  );
}
