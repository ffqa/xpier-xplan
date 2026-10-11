import { useMemo } from "react";
import { FolderPlusIcon, RefreshCwIcon } from "lucide-react";
import { toast } from "sonner";

import { Empty, ErrorBlock, LoadingBlock } from "@/components/common";
import { ProjectCard } from "@/components/project-card";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { invalidate } from "@/lib/query";
import type { Card } from "@/lib/types";
import { useOverview, useUI } from "@/lib/ui-state";

const levelRank = { crit: 3, warn: 2, info: 1, ok: 0 };

/** Landing page: every project at a glance, the ones needing you first. */
export function HomeView() {
  const ui = useUI();
  const overview = useOverview();
  const cards = overview.data?.cards ?? [];
  const sections = useMemo(() => buildSections(cards), [cards]);

  const rescanAll = async () => {
    const results = await Promise.allSettled(
      cards.map((card) => api.scan(card.slug)),
    );
    const failed = results.filter(
      (result) => result.status === "rejected",
    ).length;
    invalidate("overview", "brief:", "docs:");
    if (failed) toast.error(`${failed} scans failed`);
    else toast.success(`Rescanned ${cards.length} projects`);
  };

  if (overview.error && !overview.data)
    return <ErrorBlock error={overview.error} />;
  if (!overview.data) return <LoadingBlock />;
  if (cards.length === 0) {
    return (
      <Empty
        icon={<FolderPlusIcon />}
        title="No projects registered"
        className="h-full"
      >
        <p>
          Register the folders you work on. xplan keeps their intent, decisions
          and progress outside the code, so you can come back without the
          warm-up.
        </p>
        <Button className="mt-3" onClick={() => ui.setAddOpen(true)}>
          Register projects
        </Button>
      </Empty>
    );
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-6xl space-y-6 p-6">
        <header className="flex items-end gap-4">
          <div>
            <h1 className="text-lg font-semibold">Where everything stands</h1>
            <p className="text-[13px] text-muted-foreground">
              Projects needing your attention come first. Open one to get its
              re-entry brief.
            </p>
          </div>
          <Button
            variant="outline"
            size="sm"
            className="ml-auto"
            onClick={() => void rescanAll()}
          >
            <RefreshCwIcon />
            Rescan all
          </Button>
        </header>
        {sections.map((section) =>
          section.cards.length === 0 ? null : (
            <section key={section.title} className="space-y-2">
              <h2 className="text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">
                {section.title}{" "}
                <span className="font-normal">· {section.cards.length}</span>
              </h2>
              <div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-3">
                {section.cards.map((card) => (
                  <ProjectCard key={card.slug} card={card} />
                ))}
              </div>
            </section>
          ),
        )}
      </div>
    </div>
  );
}

function buildSections(cards: Card[]) {
  const byAttention = (a: Card, b: Card) =>
    levelRank[b.health_level] - levelRank[a.health_level] ||
    a.name.localeCompare(b.name);
  const inFlight = cards
    .filter(
      (card) =>
        card.status === "active" ||
        card.status === "waiting" ||
        card.status === "blocked",
    )
    .sort(byAttention);
  const resting = cards
    .filter((card) => card.status === "paused")
    .sort(byAttention);
  const done = cards.filter((card) => card.status === "done").sort(byAttention);
  return [
    { title: "In flight", cards: inFlight },
    { title: "Paused", cards: resting },
    { title: "Done", cards: done },
  ];
}
