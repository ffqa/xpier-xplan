import { useState } from "react";
import {
  ArrowDownIcon,
  ArrowUpIcon,
  CircleDashedIcon,
  Loader2Icon,
  PencilIcon,
  PlugZapIcon,
  PlusIcon,
  RotateCcwIcon,
  Trash2Icon,
} from "lucide-react";
import { toast } from "sonner";

import { Section } from "@/components/common";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { timeAgo } from "@/lib/format";
import { t } from "@/lib/i18n";
import { invalidate } from "@/lib/query";
import { healthVariant } from "@/lib/reviewers";
import type { ReviewerInfo } from "@/lib/types";
import { type EditorTarget, ReviewerEditor } from "@/views/reviewer-editor";

/** Reviewers in "auto" order with health, a connection test and editing. */
export function ReviewerAgents({
  reviewers,
  globalTimeout,
  onReorder,
}: {
  reviewers: ReviewerInfo[];
  globalTimeout: string;
  onReorder: (names: string[]) => void;
}) {
  const [testing, setTesting] = useState<ReadonlySet<string>>(new Set());
  const [editing, setEditing] = useState<EditorTarget | null>(null);
  const mark = (names: string[], busy: boolean) =>
    setTesting((current) => {
      const next = new Set(current);
      names.forEach((name) => (busy ? next.add(name) : next.delete(name)));
      return next;
    });

  const testOne = async (name: string) => {
    mark([name], true);
    try {
      const result = await api.testReviewer(name);
      if (result.health.status === "ok") {
        toast.success(t("{name} is working", { name }));
      } else {
        toast.warning(`${name}: ${result.health_label}`, {
          description: result.health.detail,
        });
      }
      invalidate("meta");
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      mark([name], false);
    }
  };

  const testAll = async () => {
    const names = reviewers.filter((r) => r.installed).map((r) => r.name);
    mark(names, true);
    try {
      const { results } = await api.testReviewers();
      const working = Object.values(results).filter(
        (health) => health.status === "ok",
      ).length;
      toast.success(
        t("{working} of {total} installed reviewers are working", {
          working,
          total: names.length,
        }),
      );
      invalidate("meta");
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      mark(names, false);
    }
  };

  const remove = async (reviewer: ReviewerInfo) => {
    const question = reviewer.builtin
      ? t(
          "Reset {name} to the preset? Your model, agent and command changes are dropped.",
          { name: reviewer.name },
        )
      : t("Delete {name}?", { name: reviewer.name });
    if (!window.confirm(question)) return;
    try {
      await api.removeReviewer(reviewer.name);
      toast.success(
        reviewer.builtin
          ? t("{name} reset", { name: reviewer.name })
          : t("{name} deleted", { name: reviewer.name }),
      );
      invalidate("meta");
    } catch (error) {
      toast.error((error as Error).message);
    }
  };

  const move = (index: number, delta: number) => {
    const names = reviewers.map((reviewer) => reviewer.name);
    const target = index + delta;
    if (target < 0 || target >= names.length) return;
    [names[index], names[target]] = [names[target], names[index]];
    onReorder(names);
  };

  return (
    <Section
      title={t("Reviewer agents")}
      bodyClassName="p-0"
      actions={
        <>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setEditing({ mode: "add" })}
          >
            <PlusIcon />
            {t("Add reviewer")}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            disabled={testing.size > 0}
            onClick={() => void testAll()}
          >
            {testing.size > 0 ? (
              <Loader2Icon className="animate-spin" />
            ) : (
              <PlugZapIcon />
            )}
            {t("Test all")}
          </Button>
        </>
      }
    >
      <ol className="divide-y text-[12.5px]">
        {reviewers.map((reviewer, index) => (
          <li key={reviewer.name} className="flex gap-3 px-3 py-2">
            <span className="w-4 pt-1 text-right text-muted-foreground tabular-nums">
              {index + 1}
            </span>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="font-medium">{reviewer.name}</span>
                <HealthBadge reviewer={reviewer} />
                {!reviewer.builtin && (
                  <Badge variant="outline">{t("custom")}</Badge>
                )}
                {reviewer.modified && (
                  <Badge variant="info">{t("changed")}</Badge>
                )}
                <div className="ml-auto flex items-center gap-0.5">
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={!reviewer.installed || testing.has(reviewer.name)}
                    onClick={() => void testOne(reviewer.name)}
                    title={t("Ask it to reply with a fixed word (read-only)")}
                  >
                    {testing.has(reviewer.name) && (
                      <Loader2Icon className="animate-spin" />
                    )}
                    {t("Test")}
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => setEditing({ mode: "edit", reviewer })}
                    aria-label={t("Edit")}
                    title={t("Model, agent, timeout, command")}
                  >
                    <PencilIcon />
                  </Button>
                  {(!reviewer.builtin || reviewer.modified) && (
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      onClick={() => void remove(reviewer)}
                      aria-label={reviewer.builtin ? t("Reset") : t("Delete")}
                      title={
                        reviewer.builtin ? t("Reset to preset") : t("Delete")
                      }
                    >
                      {reviewer.builtin ? <RotateCcwIcon /> : <Trash2Icon />}
                    </Button>
                  )}
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    disabled={index === 0}
                    onClick={() => move(index, -1)}
                    aria-label={t("Try earlier")}
                  >
                    <ArrowUpIcon />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    disabled={index === reviewers.length - 1}
                    onClick={() => move(index, 1)}
                    aria-label={t("Try later")}
                  >
                    <ArrowDownIcon />
                  </Button>
                </div>
              </div>
              <p className="text-muted-foreground">{reviewer.description}</p>
              <ReviewerSettings reviewer={reviewer} />
              <code className="mt-1 block font-mono text-[11px] break-all text-muted-foreground/80">
                {reviewer.command.join(" ")}
              </code>
              <HealthLine reviewer={reviewer} />
            </div>
          </li>
        ))}
      </ol>
      <p className="border-t px-3 py-2 text-xs text-muted-foreground">
        {t(
          '"auto" tries these top to bottom and skips any that is missing or not logged in. Reorder with the arrows, then Save. Edit a reviewer to pick its model, agent or timeout; "Add reviewer" takes any other CLI. Every preset runs the agent read-only.',
        )}
      </p>
      <ReviewerEditor
        target={editing}
        globalTimeout={globalTimeout}
        onClose={() => setEditing(null)}
      />
    </Section>
  );
}

/** Model, agent and own timeout, when set. */
function ReviewerSettings({ reviewer }: { reviewer: ReviewerInfo }) {
  const items = [
    reviewer.model && t("model {value}", { value: reviewer.model }),
    reviewer.agent && t("agent {value}", { value: reviewer.agent }),
    reviewer.timeout && t("timeout {value}", { value: reviewer.timeout }),
  ].filter(Boolean);
  if (items.length === 0) return null;
  return (
    <div className="mt-1 flex flex-wrap gap-1">
      {items.map((item) => (
        <Badge key={item as string} variant="secondary" className="font-mono">
          {item}
        </Badge>
      ))}
    </div>
  );
}

function HealthBadge({ reviewer }: { reviewer: ReviewerInfo }) {
  if (!reviewer.installed) {
    return <Badge variant="muted">{t("not installed")}</Badge>;
  }
  if (!reviewer.health) {
    return (
      <Badge variant="outline">
        <CircleDashedIcon />
        {t("not tested")}
      </Badge>
    );
  }
  return (
    <Badge variant={healthVariant[reviewer.health.status]}>
      {reviewer.health_label}
    </Badge>
  );
}

/** When and how the last check went, with the agent's own error line. */
function HealthLine({ reviewer }: { reviewer: ReviewerInfo }) {
  const health = reviewer.health;
  if (!reviewer.installed || !health) return null;
  const source =
    health.source === "run" ? t("during a review") : t("connection test");
  return (
    <p className="mt-1 text-[11px] text-muted-foreground">
      {t("Checked {time} ({source})", {
        time: timeAgo(health.checked_at),
        source,
      })}
      {health.detail && (
        <span className="block break-words text-muted-foreground/80">
          {health.detail}
        </span>
      )}
    </p>
  );
}
