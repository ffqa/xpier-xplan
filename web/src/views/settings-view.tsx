import { useEffect, useState } from "react";
import { CheckIcon, Loader2Icon, XIcon } from "lucide-react";
import { toast } from "sonner";

import { ErrorBlock, LoadingBlock, Section } from "@/components/common";
import { KindIcon } from "@/components/status";
import { Badge } from "@/components/ui/badge";
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
import { invalidate, useQuery } from "@/lib/query";
import type { Config } from "@/lib/types";
import { useMeta, useUI } from "@/lib/ui-state";

/** Global settings: reviewer, schedule, scan cadence. */
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
        report_language: draft.report_language,
        scan_interval: draft.scan_interval,
        review_timeout: draft.review_timeout,
        max_concurrent_reviews: Number(draft.max_concurrent_reviews),
        default_schedule: draft.default_schedule,
      });
      toast.success("Settings saved");
      invalidate("config", "meta", "overview");
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const reviewers = meta.data?.reviewers ?? [];

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-3xl space-y-4 p-6">
        <header className="flex items-center gap-3">
          <div>
            <h1 className="text-lg font-semibold">Settings</h1>
            <p className="text-[13px] text-muted-foreground">
              Stored in{" "}
              <code className="font-mono text-xs">
                {meta.data?.home}/config.yaml
              </code>
              . Projects can override reviewer and schedule.
            </p>
          </div>
          <Button
            className="ml-auto"
            disabled={!dirty || saving}
            onClick={() => void save()}
          >
            {saving && <Loader2Icon className="animate-spin" />}
            Save
          </Button>
        </header>

        <Section title="Reviews">
          <div className="grid grid-cols-[180px_1fr] items-center gap-x-4 gap-y-3">
            <Label hint="Used by projects that do not pick their own">
              Default reviewer
            </Label>
            <Select
              value={draft.default_reviewer}
              onValueChange={(value) => set("default_reviewer", String(value))}
            >
              <SelectTrigger className="w-56">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {reviewers.map((reviewer) => (
                  <SelectItem key={reviewer.name} value={reviewer.name}>
                    {reviewer.name}
                    {!reviewer.installed && (
                      <span className="text-xs text-muted-foreground">
                        (not installed)
                      </span>
                    )}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Label hint="Language the agent writes reports in">
              Report language
            </Label>
            <Input
              className="w-56"
              value={draft.report_language}
              onChange={(event) => set("report_language", event.target.value)}
            />
            <Label hint="Kill a review after this long, e.g. 20m">
              Review timeout
            </Label>
            <Input
              className="w-56"
              value={draft.review_timeout}
              onChange={(event) => set("review_timeout", event.target.value)}
            />
            <Label hint="Reviews running at the same time (1-8)">
              Parallel reviews
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
            <Label hint="How often docs, git and progress are rescanned, e.g. 15m">
              Scan interval
            </Label>
            <Input
              className="w-56"
              value={draft.scan_interval}
              onChange={(event) => set("scan_interval", event.target.value)}
            />
          </div>
        </Section>

        <Section title="Default review schedule">
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
            Copied into newly registered projects; existing projects keep their
            own schedule (project → Settings). The scheduler starts at most one
            due review per minute and skips paused and done projects.
          </p>
        </Section>

        <Section title="Reviewer agents" bodyClassName="p-0">
          <table className="w-full text-[12.5px]">
            <tbody className="divide-y">
              {reviewers.map((reviewer) => (
                <tr key={reviewer.name}>
                  <td className="w-32 px-3 py-2 align-top font-medium">
                    {reviewer.name}
                  </td>
                  <td className="px-3 py-2">
                    <p className="text-muted-foreground">
                      {reviewer.description}
                    </p>
                    <code className="mt-1 block font-mono text-[11px] break-all text-muted-foreground/80">
                      {reviewer.command.join(" ")}
                    </code>
                  </td>
                  <td className="w-28 px-3 py-2 text-right align-top">
                    {reviewer.installed ? (
                      <Badge variant="ok">
                        <CheckIcon /> installed
                      </Badge>
                    ) : (
                      <Badge variant="muted">
                        <XIcon /> missing
                      </Badge>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="border-t px-3 py-2 text-xs text-muted-foreground">
            Add or change agents in config.yaml. Every preset runs the agent
            read-only; {"{prompt}"}, {"{prompt_file}"} and {"{path}"} are
            substituted.
          </p>
        </Section>

        <Section title="Projects">
          <div className="flex items-center gap-3">
            <p className="text-[13px] text-muted-foreground">
              Register a folder or discover every git repository under a
              directory.
            </p>
            <Button
              variant="outline"
              size="sm"
              className="ml-auto"
              onClick={() => ui.setAddOpen(true)}
            >
              Register projects
            </Button>
          </div>
        </Section>
      </div>
    </div>
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
        {kind}
      </span>
      <div className="flex w-fit rounded-md border p-0.5">
        {options.map((option) => (
          <button
            key={option}
            type="button"
            onClick={() => onChange(option)}
            className={`rounded px-2.5 py-0.5 text-xs capitalize transition-colors ${value === option ? "bg-accent text-foreground" : "text-muted-foreground hover:text-foreground"}`}
          >
            {option}
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
