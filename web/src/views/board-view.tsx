import { useState } from "react";
import { toast } from "sonner";

import { ErrorBlock, LoadingBlock } from "@/components/common";
import { ProjectCard } from "@/components/project-card";
import { StatusDot } from "@/components/status";
import { api } from "@/lib/api";
import { statusHint, statusLabel } from "@/lib/format";
import { invalidate, setQueryData } from "@/lib/query";
import type { Card, Job, Status } from "@/lib/types";
import { useOverview } from "@/lib/ui-state";
import { cn } from "@/lib/utils";

const columns: Status[] = ["active", "waiting", "blocked", "paused", "done"];

type Overview = { cards: Card[]; active_reviews: Job[] };

/** Kanban of projects by status; drag a card to move it. */
export function BoardView() {
  const overview = useOverview();
  const [dragging, setDragging] = useState<string | null>(null);
  const [over, setOver] = useState<Status | null>(null);

  const move = async (slug: string, status: Status) => {
    const card = overview.data?.cards.find((item) => item.slug === slug);
    if (!card || card.status === status) return;
    const previous = card.status;
    setQueryData<Overview>("overview", (current) =>
      patchStatus(current, slug, status),
    );
    try {
      await api.patchProject(slug, { status });
      toast.success(`${card.name} → ${statusLabel[status]}`);
      invalidate("overview", `brief:${slug}`);
    } catch (error) {
      setQueryData<Overview>("overview", (current) =>
        patchStatus(current, slug, previous),
      );
      toast.error((error as Error).message);
    }
  };

  if (overview.error && !overview.data)
    return <ErrorBlock error={overview.error} />;
  if (!overview.data) return <LoadingBlock />;

  return (
    <div className="flex h-full gap-3 overflow-x-auto p-4">
      {columns.map((status) => {
        const cards = overview.data!.cards.filter(
          (card) => card.status === status,
        );
        return (
          <section
            key={status}
            onDragOver={(event) => {
              event.preventDefault();
              setOver(status);
            }}
            onDragLeave={() =>
              setOver((current) => (current === status ? null : current))
            }
            onDrop={(event) => {
              event.preventDefault();
              const slug = event.dataTransfer.getData("text/x-xplan-slug");
              setOver(null);
              setDragging(null);
              if (slug) void move(slug, status);
            }}
            className={cn(
              "flex w-72 min-w-72 flex-col rounded-lg border bg-sidebar/60 transition-colors",
              over === status && dragging && "border-primary/60 bg-primary/5",
            )}
          >
            <header
              className="flex h-10 shrink-0 items-center gap-2 border-b px-3"
              title={statusHint[status]}
            >
              <StatusDot status={status} />
              <span className="text-[12.5px] font-medium">
                {statusLabel[status]}
              </span>
              <span className="text-xs text-muted-foreground tabular-nums">
                {cards.length}
              </span>
            </header>
            <div className="min-h-0 flex-1 space-y-2 overflow-y-auto p-2">
              {cards.map((card) => (
                <ProjectCard
                  key={card.slug}
                  card={card}
                  compact
                  draggable
                  onDragStart={(event) => {
                    event.dataTransfer.setData("text/x-xplan-slug", card.slug);
                    event.dataTransfer.effectAllowed = "move";
                    setDragging(card.slug);
                  }}
                  onDragEnd={() => {
                    setDragging(null);
                    setOver(null);
                  }}
                  className={cn(
                    "cursor-grab active:cursor-grabbing",
                    dragging === card.slug && "opacity-40",
                  )}
                />
              ))}
              {cards.length === 0 && (
                <p className="p-4 text-center text-xs text-muted-foreground/70">
                  {statusHint[status]}
                </p>
              )}
            </div>
          </section>
        );
      })}
    </div>
  );
}

function patchStatus(
  current: Overview | undefined,
  slug: string,
  status: Status,
): Overview {
  if (!current) return { cards: [], active_reviews: [] };
  return {
    ...current,
    cards: current.cards.map((card) =>
      card.slug === slug ? { ...card, status } : card,
    ),
  };
}
