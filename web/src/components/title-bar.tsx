import {
  ChevronRightIcon,
  Loader2Icon,
  SearchIcon,
  TerminalSquareIcon,
} from "lucide-react";

import { KindIcon } from "@/components/status";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Kbd } from "@/components/ui/misc";
import { Tip } from "@/components/ui/tooltip";
import { duration, jobStatusLabel, kindLabel } from "@/lib/format";
import { t } from "@/lib/i18n";
import { navigate, type ProjectTab, type Route } from "@/lib/route";
import { useOverview, useUI } from "@/lib/ui-state";
import { cn } from "@/lib/utils";

const viewTitle: Record<string, string> = {
  home: "Overview",
  board: "Board",
  inbox: "Inbox",
  settings: "Settings",
};

const tabTitle: Record<ProjectTab, string> = {
  brief: "Brief",
  intent: "Intent",
  decisions: "Decisions",
  docs: "Docs",
  reviews: "Reviews",
  journal: "Journal",
  settings: "Settings",
};

export function TitleBar({ route }: { route: Route }) {
  const ui = useUI();
  const overview = useOverview();
  const projectName =
    route.view === "project"
      ? (overview.data?.cards.find((card) => card.slug === route.slug)?.name ??
        route.slug)
      : null;

  return (
    <header className="flex h-10 shrink-0 items-center gap-3 border-b bg-sidebar px-3">
      <a
        href="#/"
        className="flex items-center gap-2 font-semibold tracking-tight"
      >
        <Logo />
        <span>xplan</span>
      </a>
      <nav className="flex min-w-0 items-center gap-1 text-[12.5px] text-muted-foreground">
        <ChevronRightIcon className="size-3.5 shrink-0" />
        {projectName ? (
          <>
            <span className="truncate text-foreground">{projectName}</span>
            <ChevronRightIcon className="size-3.5 shrink-0" />
            <span className="capitalize">
              {route.view === "project" ? t(tabTitle[route.tab]) : ""}
            </span>
          </>
        ) : (
          <span className="text-foreground">{t(viewTitle[route.view])}</span>
        )}
      </nav>

      <button
        type="button"
        onClick={() => ui.setPaletteOpen(true)}
        className="mx-auto flex h-7 w-full max-w-md items-center gap-2 rounded-md border bg-background/60 px-2.5 text-[12.5px] text-muted-foreground transition-colors hover:border-ring/50 hover:text-foreground"
      >
        <SearchIcon className="size-3.5" />
        <span>{t("Jump to project, tab or command")}</span>
        <Kbd className="ml-auto">⌘K</Kbd>
      </button>

      <div className="flex items-center gap-1">
        <RunningReviews />
        <Tip label={t("Toggle console (⌘J)")} side="bottom">
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => ui.toggleConsole()}
            className={cn(ui.consoleOpen && "text-primary")}
          >
            <TerminalSquareIcon />
          </Button>
        </Tip>
      </div>
    </header>
  );
}

function RunningReviews() {
  const overview = useOverview();
  const jobs = overview.data?.active_reviews ?? [];
  if (jobs.length === 0) return null;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<Button variant="ghost" size="sm" className="text-info" />}
      >
        <Loader2Icon className="animate-spin" />
        {t("{count} running", { count: jobs.length })}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-72">
        {jobs.map((job) => (
          <DropdownMenuItem
            key={job.id}
            onClick={() =>
              navigate({
                view: "project",
                slug: job.project,
                tab: "reviews",
                arg: job.id,
              })
            }
          >
            <KindIcon kind={job.kind} />
            <span className="truncate">
              {job.project} · {kindLabel(job.kind)}
            </span>
            <span className="ml-auto text-xs text-muted-foreground">
              {job.status === "queued"
                ? jobStatusLabel(job.status)
                : duration(job.started_at)}
            </span>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function Logo() {
  return (
    <svg viewBox="0 0 24 24" className="size-5" aria-hidden>
      <rect
        x="2"
        y="2"
        width="20"
        height="20"
        rx="5"
        className="fill-primary"
      />
      <path
        d="M7 7l10 10M17 7l-4 4M7 17l4-4"
        className="stroke-primary-foreground"
        strokeWidth="2.2"
        strokeLinecap="round"
        fill="none"
      />
    </svg>
  );
}
