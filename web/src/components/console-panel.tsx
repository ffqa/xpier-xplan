import { Fragment, useEffect, useMemo, useRef, useState } from "react";
import {
  ArrowRightIcon,
  CornerDownLeftIcon,
  Loader2Icon,
  TerminalSquareIcon,
  XIcon,
} from "lucide-react";
import { toast } from "sonner";

import { Markdown } from "@/components/markdown";
import { KindIcon, VerdictBadge } from "@/components/status";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import {
  dateLocale,
  duration,
  jobActive,
  jobStatusLabel,
  kindLabel,
} from "@/lib/format";
import { t, tn } from "@/lib/i18n";
import { jobReviewer } from "@/lib/reviewers";
import { invalidate, useQuery } from "@/lib/query";
import { projectFileResolver } from "@/lib/links";
import { routeHref } from "@/lib/route";
import type { ConsoleCommand, HistoryItem } from "@/lib/types";
import { refreshProject, useMeta, useOverview, useUI } from "@/lib/ui-state";
import { cn } from "@/lib/utils";

export function ConsolePanel({ slug }: { slug: string | null }) {
  const ui = useUI();
  const overview = useOverview();
  const name = slug
    ? (overview.data?.cards.find((card) => card.slug === slug)?.name ?? slug)
    : null;

  return (
    <section className="flex h-full flex-col bg-panel">
      <header className="flex h-8 shrink-0 items-center gap-2 border-b px-3">
        <TerminalSquareIcon className="size-3.5 text-muted-foreground" />
        <span className="text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">
          {t("Console")}
        </span>
        {name && (
          <span className="text-[12px] text-muted-foreground">· {name}</span>
        )}
        <span className="ml-3 hidden text-[11px] text-muted-foreground/70 md:inline">
          {t(
            "Agents run read-only. Type /help, a /command, or just ask a question.",
          )}
        </span>
        <Button
          variant="ghost"
          size="icon-xs"
          className="ml-auto"
          onClick={() => ui.toggleConsole(false)}
        >
          <XIcon />
        </Button>
      </header>
      {slug ? (
        <ConsoleBody key={slug} slug={slug} />
      ) : (
        <div className="flex flex-1 items-center justify-center text-xs text-muted-foreground">
          {t("Select a project on the left to issue commands.")}
        </div>
      )}
    </section>
  );
}

function ConsoleBody({ slug }: { slug: string }) {
  const ui = useUI();
  const meta = useMeta();
  const history = useQuery(`console:${slug}`, () => api.consoleHistory(slug));
  const [input, setInput] = useState("");
  const [running, setRunning] = useState(false);
  const [cursor, setCursor] = useState(-1);
  const [selected, setSelected] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  const items = useMemo(
    () => [...(history.data?.items ?? [])].reverse(),
    [history.data],
  );
  const pastInputs = useMemo(
    () => (history.data?.items ?? []).map((item) => item.input),
    [history.data],
  );
  const suggestions = useMemo(
    () => matchCommands(meta.data?.commands ?? [], input),
    [meta.data, input],
  );

  useEffect(() => {
    if (!ui.consoleSeed.nonce) return;
    setInput(ui.consoleSeed.text);
    inputRef.current?.focus();
  }, [ui.consoleSeed]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [items.length]);

  useEffect(() => setSelected(0), [input]);

  const submit = async () => {
    const line = input.trim();
    if (!line || running) return;
    setRunning(true);
    try {
      const result = await api.runConsole(slug, line);
      setInput("");
      setCursor(-1);
      if (!result.ok) toast.error(result.output.split("\n")[0]);
      await history.refetch();
      refreshProject(slug);
      invalidate("inbox");
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setRunning(false);
      inputRef.current?.focus();
    }
  };

  const complete = (command: ConsoleCommand) => {
    setInput(command.name + " ");
    inputRef.current?.focus();
  };

  const onKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (suggestions.length > 0) {
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        const step = event.key === "ArrowDown" ? 1 : -1;
        setSelected(
          (current) =>
            (current + step + suggestions.length) % suggestions.length,
        );
        return;
      }
      if (
        event.key === "Tab" ||
        (event.key === "Enter" && suggestions[selected].name !== input.trim())
      ) {
        event.preventDefault();
        complete(suggestions[selected]);
        return;
      }
    }
    if (event.key === "Enter") {
      event.preventDefault();
      void submit();
    } else if (event.key === "ArrowUp" && pastInputs.length > 0) {
      event.preventDefault();
      const next = Math.min(cursor + 1, pastInputs.length - 1);
      setCursor(next);
      setInput(pastInputs[next]);
    } else if (event.key === "ArrowDown") {
      event.preventDefault();
      const next = cursor - 1;
      setCursor(Math.max(next, -1));
      setInput(next >= 0 ? pastInputs[next] : "");
    } else if (event.key === "Escape") {
      setInput("");
    }
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div
        ref={scrollRef}
        className="min-h-0 flex-1 overflow-y-auto px-3 py-2 font-mono text-[12px] leading-relaxed"
      >
        {items.length === 0 && !history.loading && (
          <p className="text-muted-foreground">
            {fillSlots(
              t(
                "No commands yet. Try {review} to check this project against its intent, or {park} before switching away.",
              ),
              {
                review: <span className="text-foreground">/review drift</span>,
                park: <span className="text-foreground">/park</span>,
              },
            )}
          </p>
        )}
        {items.map((item, index) => (
          <HistoryLine key={item.at + index} slug={slug} item={item} />
        ))}
      </div>

      <div className="relative shrink-0 border-t">
        {suggestions.length > 0 && (
          <ul className="absolute inset-x-2 bottom-full mb-1 overflow-hidden rounded-md border bg-popover py-1 shadow-lg">
            {suggestions.map((command, index) => (
              <li key={command.name}>
                <button
                  type="button"
                  onMouseDown={(event) => {
                    event.preventDefault();
                    complete(command);
                  }}
                  className={cn(
                    "flex w-full items-baseline gap-2 px-3 py-1 text-left text-[12px]",
                    index === selected && "bg-accent",
                  )}
                >
                  <span className="font-mono text-primary">{command.name}</span>
                  <span className="font-mono text-muted-foreground">
                    {command.args}
                  </span>
                  <span className="ml-auto truncate pl-4 text-muted-foreground">
                    {command.summary}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
        <div className="flex h-9 items-center gap-2 px-3">
          <span className="font-mono text-primary">›</span>
          <input
            ref={inputRef}
            value={input}
            onChange={(event) => setInput(event.target.value)}
            onKeyDown={onKeyDown}
            placeholder={t(
              "/review drift · /park stopped | next | open · or ask anything about this project",
            )}
            className="h-full flex-1 bg-transparent font-mono text-[12.5px] outline-none placeholder:text-muted-foreground/60"
            spellCheck={false}
            autoComplete="off"
          />
          {running ? (
            <Loader2Icon className="size-3.5 animate-spin text-muted-foreground" />
          ) : (
            <CornerDownLeftIcon className="size-3.5 text-muted-foreground/60" />
          )}
        </div>
      </div>
    </div>
  );
}

function HistoryLine({ slug, item }: { slug: string; item: HistoryItem }) {
  const jobIds =
    item.result.job_ids ?? (item.result.job_id ? [item.result.job_id] : []);
  return (
    <div className="mb-2">
      <div className="flex gap-2">
        <span className="text-primary">›</span>
        <span className="whitespace-pre-wrap text-foreground">
          {item.input}
        </span>
        <span className="ml-auto shrink-0 text-[10.5px] text-muted-foreground/60">
          {new Date(item.at).toLocaleTimeString(dateLocale())}
        </span>
      </div>
      <pre
        className={cn(
          "ml-4 font-mono whitespace-pre-wrap",
          item.result.ok ? "text-muted-foreground" : "text-crit",
        )}
      >
        {item.result.output}
      </pre>
      {jobIds.map((id) => (
        <JobResult key={id} slug={slug} id={id} />
      ))}
    </div>
  );
}

/** Live status of a job started from the console; ask answers render inline. */
function JobResult({ slug, id }: { slug: string; id: string }) {
  const [active, setActive] = useState(true);
  const overview = useOverview();
  const projectPath =
    overview.data?.cards.find((card) => card.slug === slug)?.path ?? "";
  const resolveHref = useMemo(
    () => projectFileResolver(slug, projectPath),
    [slug, projectPath],
  );
  const detail = useQuery(`review:${slug}:${id}`, () => api.review(slug, id), {
    refetchInterval: active ? 2500 : false,
  });
  const job = detail.data?.job;
  const finished = job !== undefined && !jobActive(job.status);
  const sawRunning = useRef(false);
  if (job && jobActive(job.status)) sawRunning.current = true;

  useEffect(() => {
    if (!finished || !active) return;
    setActive(false);
    if (sawRunning.current) refreshProject(slug);
  }, [finished, active, slug]);

  if (detail.error)
    return <p className="ml-4 text-crit">{detail.error.message}</p>;
  if (!job) return null;

  if (jobActive(job.status)) {
    return (
      <p className="ml-4 flex items-center gap-2 text-info">
        <Loader2Icon className="size-3 animate-spin" />
        {t("{kind} {status} with {reviewer}", {
          kind: kindLabel(job.kind),
          status: jobStatusLabel(job.status),
          reviewer: jobReviewer(job),
        })}{" "}
        · {duration(job.started_at ?? job.created_at)}
      </p>
    );
  }
  if (job.status !== "done") {
    const kind = kindLabel(job.kind);
    const status = jobStatusLabel(job.status);
    return (
      <p className="ml-4 text-crit">
        {job.error
          ? t("{kind} {status}: {error}", { kind, status, error: job.error })
          : t("{kind} {status}", { kind, status })}
      </p>
    );
  }
  if (job.kind === "ask") {
    return (
      <div className="mt-1 ml-4 rounded-md border bg-card/60 px-3 py-2 font-sans">
        <Markdown className="text-[13px]" resolveHref={resolveHref}>
          {detail.data?.report || t("(empty answer)")}
        </Markdown>
      </div>
    );
  }
  const target =
    job.kind === "intent"
      ? routeHref({ view: "project", slug, tab: "intent" })
      : routeHref({ view: "project", slug, tab: "reviews", arg: id });
  return (
    <a
      href={target}
      className="mt-1 ml-4 flex items-center gap-2 rounded-md border bg-card/60 px-3 py-1.5 font-sans hover:border-ring/50"
    >
      <KindIcon kind={job.kind} />
      <VerdictBadge verdict={job.verdict} kind={job.kind} />
      {typeof job.score === "number" && job.score > 0 && (
        <span className="text-xs text-muted-foreground">
          {t("score {score}", { score: job.score })}
        </span>
      )}
      <span className="min-w-0 flex-1 truncate text-[12.5px]">
        {job.summary ||
          tn(job.findings.length, "{count} finding", "{count} findings")}
      </span>
      <ArrowRightIcon className="size-3.5 text-muted-foreground" />
    </a>
  );
}

/** Replaces {name} slots in a translated sentence with React nodes. */
function fillSlots(
  template: string,
  slots: Record<string, React.ReactNode>,
): React.ReactNode[] {
  return template
    .split(/\{(\w+)\}/)
    .map((part, index) =>
      index % 2 === 1 ? (
        <Fragment key={index}>{slots[part] ?? `{${part}}`}</Fragment>
      ) : (
        part
      ),
    );
}

function matchCommands(
  commands: ConsoleCommand[],
  input: string,
): ConsoleCommand[] {
  if (!input.startsWith("/") || input.includes(" ")) return [];
  const needle = input.toLowerCase();
  return commands
    .filter(
      (command) => command.name.startsWith(needle) && command.name !== needle,
    )
    .slice(0, 8);
}
