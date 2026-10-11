import { useMemo, useState } from "react";
import {
  ChevronRightIcon,
  Loader2Icon,
  PlusIcon,
  SearchIcon,
} from "lucide-react";

import { Spinner } from "@/components/common";
import { HealthDot } from "@/components/status";
import { Button } from "@/components/ui/button";
import { Tip } from "@/components/ui/tooltip";
import { ago, statusHint, statusLabel } from "@/lib/format";
import { routeHref } from "@/lib/route";
import type { Card, Status } from "@/lib/types";
import { useOverview, useUI } from "@/lib/ui-state";
import { cn } from "@/lib/utils";

const statusOrder: Status[] = [
  "active",
  "waiting",
  "blocked",
  "paused",
  "done",
];
const levelRank = { crit: 3, warn: 2, info: 1, ok: 0 };

export function ProjectSidebar({ activeSlug }: { activeSlug: string | null }) {
  const ui = useUI();
  const overview = useOverview();
  const [filter, setFilter] = useState("");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>(() =>
    JSON.parse(localStorage.getItem("xplan.collapsed") ?? '{"done":true}'),
  );

  const groups = useMemo(
    () => groupCards(overview.data?.cards ?? [], filter),
    [overview.data, filter],
  );

  const toggleGroup = (status: Status) => {
    const next = { ...collapsed, [status]: !collapsed[status] };
    setCollapsed(next);
    localStorage.setItem("xplan.collapsed", JSON.stringify(next));
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex h-9 shrink-0 items-center gap-1 px-3">
        <span className="text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">
          Projects
        </span>
        {overview.loading && overview.data && <Spinner className="size-3" />}
        <Tip label="Register a project" side="bottom">
          <Button
            variant="ghost"
            size="icon-xs"
            className="ml-auto"
            onClick={() => ui.setAddOpen(true)}
          >
            <PlusIcon />
          </Button>
        </Tip>
      </div>
      <div className="px-2 pb-2">
        <label className="flex h-7 items-center gap-1.5 rounded-md border bg-background/50 px-2 focus-within:border-ring/60">
          <SearchIcon className="size-3.5 text-muted-foreground" />
          <input
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder="Filter"
            className="h-full w-full bg-transparent text-[12.5px] outline-none placeholder:text-muted-foreground"
          />
        </label>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto pb-3">
        {!overview.data && overview.loading && (
          <div className="flex justify-center p-6">
            <Spinner />
          </div>
        )}
        {overview.data && overview.data.cards.length === 0 && (
          <div className="p-4 text-center text-xs text-muted-foreground">
            No projects yet.
            <Button
              variant="link"
              size="xs"
              onClick={() => ui.setAddOpen(true)}
            >
              Register one
            </Button>
          </div>
        )}
        {groups.map(({ status, cards }) => (
          <div key={status} className="mb-1">
            <button
              type="button"
              title={statusHint[status]}
              onClick={() => toggleGroup(status)}
              className="flex h-6 w-full items-center gap-1 px-2 text-[11px] font-medium tracking-wide text-muted-foreground uppercase hover:text-foreground"
            >
              <ChevronRightIcon
                className={cn(
                  "size-3 transition-transform",
                  !collapsed[status] && "rotate-90",
                )}
              />
              {statusLabel[status]}
              <span className="ml-auto font-normal tabular-nums">
                {cards.length}
              </span>
            </button>
            {!collapsed[status] &&
              cards.map((card) => (
                <ProjectItem
                  key={card.slug}
                  card={card}
                  active={card.slug === activeSlug}
                />
              ))}
          </div>
        ))}
      </div>
    </div>
  );
}

function ProjectItem({ card, active }: { card: Card; active: boolean }) {
  const subtitle =
    card.last_park?.next ||
    card.last_park?.stopped ||
    card.summary ||
    card.path;
  return (
    <a
      href={routeHref({ view: "project", slug: card.slug, tab: "brief" })}
      className={cn(
        "group mx-1 flex flex-col gap-0.5 rounded-md px-2 py-1.5 transition-colors hover:bg-accent/60",
        active && "bg-accent text-accent-foreground hover:bg-accent",
      )}
    >
      <div className="flex items-center gap-2">
        <HealthDot level={card.health_level} />
        <span className="min-w-0 flex-1 truncate text-[13px] font-medium">
          {card.name}
        </span>
        {card.active_jobs > 0 ? (
          <Loader2Icon className="size-3 animate-spin text-info" />
        ) : (
          <span
            className="text-[11px] text-muted-foreground tabular-nums"
            title="Last commit"
          >
            {ago(card.last_commit_at)}
          </span>
        )}
      </div>
      <p
        className="truncate pl-4 text-[11.5px] text-muted-foreground"
        title={subtitle}
      >
        {card.last_park?.next ? `→ ${subtitle}` : subtitle}
      </p>
    </a>
  );
}

function groupCards(cards: Card[], filter: string) {
  const needle = filter.trim().toLowerCase();
  const visible = needle
    ? cards.filter((card) =>
        [card.name, card.slug, card.summary, ...(card.tags ?? [])].some(
          (value) => value?.toLowerCase().includes(needle),
        ),
      )
    : cards;
  return statusOrder
    .map((status) => ({
      status,
      cards: visible
        .filter((card) => card.status === status)
        .sort(
          (a, b) =>
            levelRank[b.health_level] - levelRank[a.health_level] ||
            a.name.localeCompare(b.name),
        ),
    }))
    .filter((group) => group.cards.length > 0);
}
