import {
  GitBranchIcon,
  ListChecksIcon,
  Loader2Icon,
  UsersIcon,
} from "lucide-react";

import { HealthDot, StatusDot, VerdictDots } from "@/components/status";
import { Progress } from "@/components/ui/misc";
import { ago, levelText, percent, statusLabel } from "@/lib/format";
import { routeHref } from "@/lib/route";
import type { Card } from "@/lib/types";
import { cn } from "@/lib/utils";

/** Project summary card used by the overview grid and the board. */
export function ProjectCard({
  card,
  compact,
  className,
  ...rest
}: {
  card: Card;
  compact?: boolean;
  className?: string;
} & React.ComponentProps<"a">) {
  const topFlag = card.health.find(
    (flag) => flag.level === "crit" || flag.level === "warn",
  );
  const checklist = card.checklist ?? { done: 0, total: 0 };
  return (
    <a
      href={routeHref({ view: "project", slug: card.slug, tab: "brief" })}
      className={cn(
        "group flex flex-col gap-2 rounded-lg border bg-card p-3 transition-colors hover:border-ring/50",
        className,
      )}
      {...rest}
    >
      <div className="flex items-center gap-2">
        <HealthDot level={card.health_level} />
        <span className="min-w-0 flex-1 truncate font-medium">{card.name}</span>
        {card.active_jobs > 0 && (
          <Loader2Icon className="size-3.5 animate-spin text-info" />
        )}
        <VerdictDots verdicts={card.verdicts ?? {}} />
      </div>

      {!compact && card.summary && (
        <p className="line-clamp-2 text-xs text-muted-foreground">
          {card.summary}
        </p>
      )}

      {card.last_park ? (
        <div className="rounded-md bg-muted/50 px-2 py-1.5 text-[12px] leading-snug">
          <span className="text-muted-foreground">
            Parked {ago(card.last_park.at)} ago ·{" "}
          </span>
          {card.last_park.next ? (
            <>
              <span className="text-muted-foreground">next: </span>
              {card.last_park.next}
            </>
          ) : (
            card.last_park.stopped
          )}
        </div>
      ) : (
        !compact && (
          <div className="rounded-md border border-dashed px-2 py-1.5 text-[12px] text-muted-foreground">
            No park note yet
          </div>
        )
      )}

      {checklist.total > 0 && (
        <div
          className="flex items-center gap-2"
          title={`${checklist.done}/${checklist.total} checklist items done across docs`}
        >
          <ListChecksIcon className="size-3 text-muted-foreground" />
          <Progress
            value={percent(checklist.done, checklist.total)}
            className="h-1"
          />
          <span className="text-[11px] text-muted-foreground tabular-nums">
            {percent(checklist.done, checklist.total)}%
          </span>
        </div>
      )}

      {topFlag && (
        <p className={cn("truncate text-[11.5px]", levelText[topFlag.level])}>
          {topFlag.message}
        </p>
      )}

      <div className="mt-auto flex items-center gap-3 text-[11px] text-muted-foreground">
        {!compact && (
          <span className="flex items-center gap-1">
            <StatusDot status={card.status} />
            {statusLabel[card.status]}
          </span>
        )}
        {card.branch && (
          <span className="flex min-w-0 items-center gap-1">
            <GitBranchIcon className="size-3" />
            <span className="truncate">{card.branch}</span>
          </span>
        )}
        {card.xteam_open > 0 && (
          <span className="flex items-center gap-1" title="Open xteam slices">
            <UsersIcon className="size-3" />
            {card.xteam_open}
          </span>
        )}
        <span className="ml-auto" title="Last commit">
          {ago(card.last_commit_at)}
        </span>
      </div>
    </a>
  );
}
