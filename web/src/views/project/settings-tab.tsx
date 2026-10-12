import { useEffect, useState } from "react";
import { Loader2Icon, Trash2Icon } from "lucide-react";
import { toast } from "sonner";

import { Section } from "@/components/common";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";
import { t } from "@/lib/i18n";
import { invalidate } from "@/lib/query";
import { AUTO_REVIEWER, reviewerLabel } from "@/lib/reviewers";
import { navigate } from "@/lib/route";
import type { Brief, Project } from "@/lib/types";
import { refreshProject, useMeta } from "@/lib/ui-state";
import { ReviewerSelect, ScheduleRow } from "@/views/settings-view";

interface Draft {
  name: string;
  summary: string;
  tags: string;
  reviewer: string;
  schedule: Record<string, string>;
  doc_globs: string;
}

function toDraft(project: Project): Draft {
  return {
    name: project.name,
    summary: project.summary ?? "",
    tags: (project.tags ?? []).join(", "),
    reviewer: project.reviewer ?? "",
    schedule: { ...(project.schedule ?? {}) },
    doc_globs: (project.doc_globs ?? []).join("\n"),
  };
}

const splitList = (value: string, separator: RegExp) =>
  value
    .split(separator)
    .map((item) => item.trim())
    .filter(Boolean);

/** Per-project overrides: name, reviewer, schedule, extra doc globs. */
export function ProjectSettingsTab({
  slug,
  brief,
}: {
  slug: string;
  brief: Brief;
}) {
  const meta = useMeta();
  const initial = toDraft(brief.card);
  const [draft, setDraft] = useState(initial);
  const [saving, setSaving] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState(false);
  const initialKey = JSON.stringify(initial);
  useEffect(() => setDraft(JSON.parse(initialKey) as Draft), [initialKey]);

  const dirty = JSON.stringify(draft) !== initialKey;
  const set = <K extends keyof Draft>(key: K, value: Draft[K]) =>
    setDraft((current) => ({ ...current, [key]: value }));

  const save = async () => {
    setSaving(true);
    try {
      await api.patchProject(slug, {
        name: draft.name.trim(),
        summary: draft.summary.trim(),
        tags: splitList(draft.tags, /,/),
        reviewer: draft.reviewer,
        schedule: draft.schedule,
        doc_globs: splitList(draft.doc_globs, /\n/),
      });
      toast.success(t("Project settings saved"));
      refreshProject(slug);
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    try {
      await api.removeProject(slug);
      toast.success(
        t("{name} unregistered. Its notes were moved to the archive folder.", {
          name: brief.card.name,
        }),
      );
      invalidate("overview");
      navigate({ view: "home" });
    } catch (error) {
      toast.error((error as Error).message);
    }
  };

  const reviewers = meta.data?.reviewers ?? [];
  const defaultReviewer = meta.data?.default_reviewer ?? AUTO_REVIEWER;

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-3xl space-y-4 p-5">
        <div className="flex items-center">
          <p className="text-[13px] text-muted-foreground">
            {t(
              "Settings for this project only. Empty values fall back to the global settings.",
            )}
          </p>
          <Button
            className="ml-auto"
            disabled={!dirty || saving}
            onClick={() => void save()}
          >
            {saving && <Loader2Icon className="animate-spin" />}
            {t("Save")}
          </Button>
        </div>

        <Section title={t("General")}>
          <div className="grid grid-cols-[160px_1fr] items-center gap-x-4 gap-y-3">
            <span className="text-[13px]">{t("Name")}</span>
            <Input
              value={draft.name}
              onChange={(event) => set("name", event.target.value)}
            />
            <span className="self-start pt-1.5 text-[13px]">
              {t("Summary")}
            </span>
            <Textarea
              value={draft.summary}
              onChange={(event) => set("summary", event.target.value)}
              rows={2}
            />
            <span className="text-[13px]">{t("Tags")}</span>
            <Input
              value={draft.tags}
              onChange={(event) => set("tags", event.target.value)}
              placeholder="laravel, client-a, miniapp"
            />
          </div>
        </Section>

        <Section title={t("Reviews")}>
          <div className="grid grid-cols-[160px_1fr] items-center gap-x-4 gap-y-2">
            <span className="text-[13px]">{t("Reviewer")}</span>
            <ReviewerSelect
              value={draft.reviewer}
              reviewers={reviewers}
              onChange={(value) => set("reviewer", value)}
              inherit={t("Default ({reviewer})", {
                reviewer: reviewerLabel(defaultReviewer, meta.data),
              })}
            />
            {(meta.data?.scheduled_kinds ?? []).map((kind) => (
              <ScheduleRow
                key={kind}
                kind={kind}
                value={draft.schedule[kind] ?? "off"}
                options={meta.data?.schedule_values ?? []}
                onChange={(value) =>
                  set("schedule", { ...draft.schedule, [kind]: value })
                }
              />
            ))}
          </div>
        </Section>

        <Section title={t("Extra doc globs")}>
          <Textarea
            value={draft.doc_globs}
            onChange={(event) => set("doc_globs", event.target.value)}
            rows={3}
            placeholder={"notes/**/*.md\nplans/*.txt"}
            className="font-mono text-[12px]"
          />
          <p className="mt-2 text-xs text-muted-foreground">
            {t(
              "One glob per line, relative to the project root. Added on top of the built-in doc detection.",
            )}
          </p>
        </Section>

        <Section title={t("Danger zone")} className="border-crit/30">
          <div className="flex items-center gap-3">
            <p className="text-[13px] text-muted-foreground">
              {t(
                "Unregister removes the project from xplan. The project folder is untouched; xplan's notes move to the archive folder.",
              )}
            </p>
            <Button
              variant="destructive"
              size="sm"
              className="ml-auto"
              onClick={() => setConfirmRemove(true)}
            >
              <Trash2Icon />
              {t("Unregister")}
            </Button>
          </div>
        </Section>
      </div>

      <Dialog open={confirmRemove} onOpenChange={setConfirmRemove}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("Unregister {name}?", { name: brief.card.name })}
            </DialogTitle>
            <DialogDescription>
              {t(
                "The code is not touched. Intent, decisions, journal and reviews are archived, not deleted.",
              )}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setConfirmRemove(false)}>
              {t("Keep it")}
            </Button>
            <Button variant="destructive" onClick={() => void remove()}>
              {t("Unregister")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
