import { useMemo, useState } from "react";
import { ArrowRightIcon, ChevronRightIcon, InboxIcon } from "lucide-react";

import { Empty, ErrorBlock, LoadingBlock } from "@/components/common";
import { KindIcon, SeverityBadge } from "@/components/status";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { api } from "@/lib/api";
import { ago, kindLabel, severityLabel } from "@/lib/format";
import { t } from "@/lib/i18n";
import { useQuery } from "@/lib/query";
import { routeHref } from "@/lib/route";
import type { InboxItem, Severity } from "@/lib/types";
import { cn } from "@/lib/utils";

const severities: Severity[] = ["critical", "high", "medium", "low", "info"];

function atOrAbove(severity: string) {
  const label = severities.includes(severity as Severity)
    ? severityLabel(severity as Severity)
    : severity;
  return t("{severity} and above", { severity: label });
}

/** Findings from the latest finished review of every kind, across projects. */
export function InboxView() {
  const [minimum, setMinimum] = useState(
    () => localStorage.getItem("xplan.inboxMin") ?? "medium",
  );
  const [project, setProject] = useState("all");
  const inbox = useQuery(`inbox:${minimum}`, () => api.inbox(minimum), {
    refetchInterval: 30000,
  });
  const items = inbox.data?.items ?? [];
  const projects = useMemo(
    () =>
      Array.from(
        new Map(
          items.map((item) => [item.project, item.project_name]),
        ).entries(),
      ),
    [items],
  );
  const visible =
    project === "all"
      ? items
      : items.filter((item) => item.project === project);

  const changeMinimum = (value: string) => {
    setMinimum(value);
    localStorage.setItem("xplan.inboxMin", value);
  };

  return (
    <div className="flex h-full flex-col">
      <header className="flex h-12 shrink-0 items-center gap-3 border-b px-4">
        <h1 className="font-semibold">{t("Inbox")}</h1>
        <span className="text-xs text-muted-foreground">
          {t(
            "Findings from the latest review of each kind. Re-running a review replaces its findings.",
          )}
        </span>
        <div className="ml-auto flex items-center gap-2">
          <Select
            value={project}
            onValueChange={(value) => setProject(String(value))}
          >
            <SelectTrigger size="sm" className="min-w-36">
              <SelectValue>
                {(value: string) =>
                  value === "all"
                    ? t("All projects")
                    : (projects.find(([slug]) => slug === value)?.[1] ?? value)
                }
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("All projects")}</SelectItem>
              {projects.map(([slug, name]) => (
                <SelectItem key={slug} value={slug}>
                  {name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select
            value={minimum}
            onValueChange={(value) => changeMinimum(String(value))}
          >
            <SelectTrigger size="sm" className="min-w-32">
              <SelectValue>{(value: string) => atOrAbove(value)}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              {severities.map((severity) => (
                <SelectItem key={severity} value={severity}>
                  {atOrAbove(severity)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {inbox.error && !inbox.data && <ErrorBlock error={inbox.error} />}
        {!inbox.data && !inbox.error && <LoadingBlock />}
        {inbox.data && visible.length === 0 && (
          <Empty
            icon={<InboxIcon />}
            title={t("Inbox zero")}
            className="h-full"
          >
            {t(
              "Nothing at this severity. Reviews run on each project's schedule, or start one from a project's Reviews tab.",
            )}
          </Empty>
        )}
        <ul className="divide-y">
          {visible.map((item, index) => (
            <InboxRow key={`${item.job_id}-${index}`} item={item} />
          ))}
        </ul>
      </div>
    </div>
  );
}

function InboxRow({ item }: { item: InboxItem }) {
  const [open, setOpen] = useState(false);
  const { finding } = item;
  return (
    <li>
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-3 px-4 py-2 text-left hover:bg-accent/40"
      >
        <ChevronRightIcon
          className={cn(
            "size-3.5 shrink-0 text-muted-foreground transition-transform",
            open && "rotate-90",
          )}
        />
        <SeverityBadge severity={finding.severity} />
        <span className="min-w-0 flex-1 truncate text-[13px]">
          {finding.title}
        </span>
        {finding.file && (
          <span className="hidden max-w-64 truncate font-mono text-[11px] text-muted-foreground lg:inline">
            {finding.file}
          </span>
        )}
        <span className="flex shrink-0 items-center gap-1.5 text-xs text-muted-foreground">
          <KindIcon kind={item.kind} className="size-3" />
          {item.project_name}
        </span>
        <span className="w-10 shrink-0 text-right text-xs text-muted-foreground">
          {ago(item.at)}
        </span>
      </button>
      {open && (
        <div className="space-y-2 bg-muted/20 px-4 py-3 pl-11 text-[12.5px]">
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
          <a
            href={routeHref({
              view: "project",
              slug: item.project,
              tab: "reviews",
              arg: item.job_id,
            })}
            className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
          >
            {t("Open the full {kind} report", { kind: kindLabel(item.kind) })}{" "}
            <ArrowRightIcon className="size-3" />
          </a>
        </div>
      )}
    </li>
  );
}
