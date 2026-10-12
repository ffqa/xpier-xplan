import { useMemo, useState } from "react";
import {
  FolderPlusIcon,
  Loader2Icon,
  RefreshCwIcon,
  WandSparklesIcon,
} from "lucide-react";
import { toast } from "sonner";

import { Empty, ErrorBlock, LoadingBlock } from "@/components/common";
import { ProjectCard } from "@/components/project-card";
import { Button } from "@/components/ui/button";
import { Tip } from "@/components/ui/tooltip";
import { api } from "@/lib/api";
import { t, tn } from "@/lib/i18n";
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

  const [drafting, setDrafting] = useState(false);
  const draftable = cards.filter(
    (card) => !card.intent_filled && card.established,
  ).length;

  const draftMissing = async () => {
    setDrafting(true);
    try {
      const result = await api.draftIntents();
      const fresh = result.skipped.filter((item) => item.code === "new").length;
      const queuedMessage = tn(
        result.queued.length,
        "Drafting {count} intent (read-only).",
        "Drafting {count} intents (read-only).",
      );
      toast.success(
        fresh
          ? t("{queued} {fresh}", {
              queued: queuedMessage,
              fresh: tn(
                fresh,
                "{count} new project is left for you to write.",
                "{count} new projects are left for you to write.",
              ),
            })
          : queuedMessage,
      );
      invalidate("overview", "reviews:");
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setDrafting(false);
    }
  };

  const rescanAll = async () => {
    const results = await Promise.allSettled(
      cards.map((card) => api.scan(card.slug)),
    );
    const failed = results.filter(
      (result) => result.status === "rejected",
    ).length;
    invalidate("overview", "brief:", "docs:");
    if (failed)
      toast.error(tn(failed, "{count} scan failed", "{count} scans failed"));
    else
      toast.success(
        tn(
          cards.length,
          "Rescanned {count} project",
          "Rescanned {count} projects",
        ),
      );
  };

  if (overview.error && !overview.data)
    return <ErrorBlock error={overview.error} />;
  if (!overview.data) return <LoadingBlock />;
  if (cards.length === 0) {
    return (
      <Empty
        icon={<FolderPlusIcon />}
        title={t("No projects registered")}
        className="h-full"
      >
        <p>
          {t(
            "Register the folders you work on. xplan keeps their intent, decisions and progress outside the code, so you can come back without the warm-up.",
          )}
        </p>
        <Button className="mt-3" onClick={() => ui.setAddOpen(true)}>
          {t("Register projects")}
        </Button>
      </Empty>
    );
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-6xl space-y-6 p-6">
        <header className="flex items-end gap-4">
          <div>
            <h1 className="text-lg font-semibold">
              {t("Where everything stands")}
            </h1>
            <p className="text-[13px] text-muted-foreground">
              {t(
                "Projects needing your attention come first. Open one to get its re-entry brief.",
              )}
            </p>
          </div>
          {draftable > 0 && (
            <Tip
              label={t(
                "Projects that existed before you registered them, and still have no intent: the reviewer reconstructs why each was started, read-only. You correct the drafts. New projects are left for you to write.",
              )}
            >
              <Button
                variant="outline"
                size="sm"
                className="ml-auto"
                disabled={drafting}
                onClick={() => void draftMissing()}
              >
                {drafting ? (
                  <Loader2Icon className="animate-spin" />
                ) : (
                  <WandSparklesIcon />
                )}
                {t("Draft missing intents ({count})", { count: draftable })}
              </Button>
            </Tip>
          )}
          <Button
            variant="outline"
            size="sm"
            className={draftable > 0 ? undefined : "ml-auto"}
            onClick={() => void rescanAll()}
          >
            <RefreshCwIcon />
            {t("Rescan all")}
          </Button>
        </header>
        {sections.map((section) =>
          section.cards.length === 0 ? null : (
            <section key={section.title} className="space-y-2">
              <h2 className="text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">
                {t(section.title)}{" "}
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
