import { useEffect, useState } from "react";
import { Loader2Icon } from "lucide-react";
import { toast } from "sonner";

import { ErrorBlock, LoadingBlock, Section } from "@/components/common";
import { KindIcon } from "@/components/status";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { api } from "@/lib/api";
import { kindLabel } from "@/lib/format";
import { LANGUAGES, t, useLanguage } from "@/lib/i18n";
import { switchLanguage } from "@/lib/language";
import { invalidate, useQuery } from "@/lib/query";
import { AUTO_REVIEWER, orderReviewers, reviewerLabel } from "@/lib/reviewers";
import type { Config, ReviewerInfo } from "@/lib/types";
import { useMeta, useUI } from "@/lib/ui-state";
import { ReviewerAgents } from "@/views/reviewer-agents";

/** Report languages offered as one-click presets; any other text works too. */
const REPORT_LANGUAGES = ["English", "Simplified Chinese"];

/** Global settings: language, reviewers, schedule, scan cadence. */
export function SettingsView() {
  const meta = useMeta();
  const config = useQuery("config", api.config);
  if (config.error && !config.data) return <ErrorBlock error={config.error} />;
  if (!config.data || !meta.data) return <LoadingBlock />;
  return <SettingsForm initial={config.data} />;
}

function SettingsForm({ initial }: { initial: Config }) {
  const ui = useUI();
  const meta = useMeta();
  const [draft, setDraft] = useState(initial);
  const [saving, setSaving] = useState(false);
  useEffect(() => setDraft(initial), [initial]);

  const dirty = JSON.stringify(draft) !== JSON.stringify(initial);
  const set = <K extends keyof Config>(key: K, value: Config[K]) =>
    setDraft((current) => ({ ...current, [key]: value }));

  const save = async () => {
    setSaving(true);
    try {
      await api.patchConfig({
        default_reviewer: draft.default_reviewer,
        // Reviewers added or deleted since the page loaded are reflected here.
        reviewer_order: reviewers.map((reviewer) => reviewer.name),
        report_language: draft.report_language,
        scan_interval: draft.scan_interval,
        review_timeout: draft.review_timeout,
        max_concurrent_reviews: Number(draft.max_concurrent_reviews),
        default_schedule: draft.default_schedule,
      });
      toast.success(t("Settings saved"));
      invalidate("config", "meta", "overview");
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const reviewers = orderReviewers(
    meta.data?.reviewers ?? [],
    draft.reviewer_order ?? [],
  );

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-3xl space-y-4 p-6">
        <header className="flex items-center gap-3">
          <div>
            <h1 className="text-lg font-semibold">{t("Settings")}</h1>
            <p className="text-[13px] text-muted-foreground">
              {t("Stored in")}{" "}
              <code className="font-mono text-xs">
                {meta.data?.home}/config.yaml
              </code>
              {t(". Projects can override reviewer and schedule.")}
            </p>
          </div>
          <Button
            className="ml-auto"
            disabled={!dirty || saving}
            onClick={() => void save()}
          >
            {saving && <Loader2Icon className="animate-spin" />}
            {t("Save")}
          </Button>
        </header>

        <Section title={t("Language")}>
          <div className="grid grid-cols-[180px_1fr] items-center gap-x-4 gap-y-3">
            <Label hint={t("Applies at once, also to console replies")}>
              {t("Interface")}
            </Label>
            <InterfaceLanguage />
            <Label hint={t("Language the agent writes reports in")}>
              {t("Reports")}
            </Label>
            <div className="flex flex-wrap items-center gap-2">
              <Input
                className="w-56"
                value={draft.report_language}
                onChange={(event) => set("report_language", event.target.value)}
              />
              {REPORT_LANGUAGES.map((language) => (
                <Button
                  key={language}
                  variant={
                    draft.report_language === language ? "secondary" : "ghost"
                  }
                  size="sm"
                  onClick={() => set("report_language", language)}
                >
                  {t(language)}
                </Button>
              ))}
            </div>
          </div>
        </Section>

        <Section title={t("Reviews")}>
          <div className="grid grid-cols-[180px_1fr] items-center gap-x-4 gap-y-3">
            <Label
              hint={
                draft.default_reviewer === AUTO_REVIEWER
                  ? t("Now: {choice}", {
                      choice: reviewerLabel(AUTO_REVIEWER, meta.data),
                    })
                  : t("Used by projects that do not pick their own")
              }
            >
              {t("Default reviewer")}
            </Label>
            <ReviewerSelect
              value={draft.default_reviewer}
              reviewers={reviewers}
              onChange={(value) => set("default_reviewer", value)}
            />
            <Label hint={t("Kill a review after this long, e.g. 20m")}>
              {t("Review timeout")}
            </Label>
            <Input
              className="w-56"
              value={draft.review_timeout}
              onChange={(event) => set("review_timeout", event.target.value)}
            />
            <Label hint={t("Reviews running at the same time (1-8)")}>
              {t("Parallel reviews")}
            </Label>
            <Input
              className="w-56"
              type="number"
              min={1}
              max={8}
              value={draft.max_concurrent_reviews}
              onChange={(event) =>
                set("max_concurrent_reviews", Number(event.target.value))
              }
            />
            <Label
              hint={t(
                "How often docs, git and progress are rescanned, e.g. 15m",
              )}
            >
              {t("Scan interval")}
            </Label>
            <Input
              className="w-56"
              value={draft.scan_interval}
              onChange={(event) => set("scan_interval", event.target.value)}
            />
          </div>
        </Section>

        <ReviewerAgents
          reviewers={reviewers}
          globalTimeout={initial.review_timeout}
          onReorder={(names) => set("reviewer_order", names)}
        />

        <Section title={t("Default review schedule")}>
          <div className="grid grid-cols-[180px_1fr] items-center gap-x-4 gap-y-2">
            {(meta.data?.scheduled_kinds ?? []).map((kind) => (
              <ScheduleRow
                key={kind}
                kind={kind}
                value={draft.default_schedule[kind] ?? "off"}
                options={meta.data?.schedule_values ?? []}
                onChange={(value) =>
                  set("default_schedule", {
                    ...draft.default_schedule,
                    [kind]: value,
                  })
                }
              />
            ))}
          </div>
          <p className="mt-3 text-xs text-muted-foreground">
            {t(
              "Copied into newly registered projects; existing projects keep their own schedule (project → Settings). The scheduler starts at most one due review per minute and skips paused and done projects.",
            )}
          </p>
        </Section>

        <Section title={t("Projects")}>
          <div className="flex items-center gap-3">
            <p className="text-[13px] text-muted-foreground">
              {t(
                "Register a folder or discover every git repository under a directory.",
              )}
            </p>
            <Button
              variant="outline"
              size="sm"
              className="ml-auto"
              onClick={() => ui.setAddOpen(true)}
            >
              {t("Register projects")}
            </Button>
          </div>
        </Section>
      </div>
    </div>
  );
}

/** English / 简体中文 toggle; saved on the server right away. */
function InterfaceLanguage() {
  const language = useLanguage();
  const [switching, setSwitching] = useState(false);
  const choose = async (next: (typeof LANGUAGES)[number]["id"]) => {
    if (next === language) return;
    setSwitching(true);
    try {
      await switchLanguage(next);
    } catch (error) {
      toast.error((error as Error).message);
      setSwitching(false);
    }
  };
  return (
    <div className="flex w-fit rounded-md border p-0.5">
      {LANGUAGES.map((option) => (
        <button
          key={option.id}
          type="button"
          disabled={switching}
          onClick={() => void choose(option.id)}
          className={`rounded px-2.5 py-0.5 text-xs transition-colors ${language === option.id ? "bg-accent text-foreground" : "text-muted-foreground hover:text-foreground"}`}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

/** Reviewer picker with "auto" first; shared by global and project settings. */
export function ReviewerSelect({
  value,
  reviewers,
  onChange,
  inherit,
}: {
  value: string;
  reviewers: ReviewerInfo[];
  onChange: (value: string) => void;
  /** Label of the extra "use the global default" item (project settings). */
  inherit?: string;
}) {
  const meta = useMeta();
  const inheritValue = "__default";
  const describe = (name: string) =>
    name === inheritValue
      ? (inherit ?? "")
      : name === AUTO_REVIEWER
        ? reviewerLabel(AUTO_REVIEWER, meta.data)
        : name;
  return (
    <Select
      value={value || (inherit ? inheritValue : AUTO_REVIEWER)}
      onValueChange={(next) =>
        onChange(next === inheritValue ? "" : String(next))
      }
    >
      <SelectTrigger className="w-64">
        <SelectValue>{(selected: string) => describe(selected)}</SelectValue>
      </SelectTrigger>
      <SelectContent>
        {inherit && <SelectItem value={inheritValue}>{inherit}</SelectItem>}
        <SelectItem value={AUTO_REVIEWER}>
          {t("auto")}
          <span className="text-xs text-muted-foreground">
            {t("first working reviewer")}
          </span>
        </SelectItem>
        {reviewers.map((reviewer) => (
          <SelectItem key={reviewer.name} value={reviewer.name}>
            {reviewer.name}
            {!reviewer.installed ? (
              <span className="text-xs text-muted-foreground">
                {t("(not installed)")}
              </span>
            ) : (
              reviewer.health?.status &&
              reviewer.health.status !== "ok" && (
                <span className="text-xs text-warn">
                  ({reviewer.health_label})
                </span>
              )
            )}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

export function ScheduleRow({
  kind,
  value,
  options,
  onChange,
}: {
  kind: string;
  value: string;
  options: string[];
  onChange: (value: string) => void;
}) {
  return (
    <>
      <span className="flex items-center gap-2 text-[13px] capitalize">
        <KindIcon kind={kind} className="text-muted-foreground" />
        {kindLabel(kind)}
      </span>
      <div className="flex w-fit rounded-md border p-0.5">
        {options.map((option) => (
          <button
            key={option}
            type="button"
            onClick={() => onChange(option)}
            className={`rounded px-2.5 py-0.5 text-xs capitalize transition-colors ${value === option ? "bg-accent text-foreground" : "text-muted-foreground hover:text-foreground"}`}
          >
            {t(option)}
          </button>
        ))}
      </div>
    </>
  );
}

function Label({
  children,
  hint,
}: {
  children: React.ReactNode;
  hint?: string;
}) {
  return (
    <div>
      <div className="text-[13px]">{children}</div>
      {hint && <div className="text-[11px] text-muted-foreground">{hint}</div>}
    </div>
  );
}
