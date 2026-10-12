import { useState } from "react";
import {
  CircleStopIcon,
  Loader2Icon,
  TargetIcon,
  TriangleAlertIcon,
  WandSparklesIcon,
  XIcon,
} from "lucide-react";
import { toast } from "sonner";

import { ErrorBlock, LoadingBlock } from "@/components/common";
import { MarkdownEditor } from "@/components/markdown-editor";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tip } from "@/components/ui/tooltip";
import { api } from "@/lib/api";
import { ago } from "@/lib/format";
import { useIntentDraft } from "@/lib/intent-draft";
import { setQueryData, useQuery } from "@/lib/query";
import type { Intent, Job } from "@/lib/types";
import { refreshProject } from "@/lib/ui-state";
import { cn } from "@/lib/utils";

const dismissKey = (slug: string) => `xplan.intentDraftDismissed.${slug}`;

/** intent.md: why the project exists. Owned by xplan, never by the project. */
export function IntentTab({ slug }: { slug: string }) {
  const intent = useQuery(`intent:${slug}`, () => api.intent(slug));
  const draft = useIntentDraft(slug);
  const [saving, setSaving] = useState(false);
  const [replacement, setReplacement] = useState<{
    content: string;
    nonce: number;
  }>();
  const [dismissed, setDismissed] = useState(() =>
    localStorage.getItem(dismissKey(slug)),
  );

  const dismiss = (jobId: string) => {
    localStorage.setItem(dismissKey(slug), jobId);
    setDismissed(jobId);
  };

  const save = async (content: string) => {
    setSaving(true);
    try {
      const saved = await api.saveIntent(slug, content);
      setQueryData(`intent:${slug}`, () => saved);
      toast.success(saveMessage(saved));
      refreshProject(slug);
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setSaving(false);
    }
  };

  if (intent.error && !intent.data) return <ErrorBlock error={intent.error} />;
  if (!intent.data) return <LoadingBlock />;

  const job = draft.job;
  const keptDraft =
    job?.status === "done" && !job.intent_written && dismissed !== job.id
      ? job
      : undefined;
  const failedDraft =
    job?.status === "failed" && dismissed !== job.id ? job : undefined;

  return (
    <MarkdownEditor
      storageKey="intent"
      value={intent.data.content}
      saving={saving}
      onSave={save}
      replacement={replacement}
      toolbarLeft={
        <>
          <TargetIcon className="size-3.5 text-muted-foreground" />
          <span className="text-[12.5px] font-medium">intent.md</span>
          <IntentBadge intent={intent.data} />
          <Tip label="Let the reviewer reconstruct why this project was started, from its README, docs and git history. Read-only; your own text is never overwritten.">
            <Button
              variant="outline"
              size="xs"
              disabled={draft.running || draft.starting}
              onClick={() => void draft.start()}
            >
              {draft.starting ? (
                <Loader2Icon className="animate-spin" />
              ) : (
                <WandSparklesIcon />
              )}
              Draft from history
            </Button>
          </Tip>
        </>
      }
      banner={
        <>
          {draft.running && job && (
            <Banner tone="info" icon={<Loader2Icon className="animate-spin" />}>
              <span>
                {job.status === "queued"
                  ? "Waiting for a free review slot…"
                  : `${job.reviewer} is reading the README, docs and git history (read-only).`}{" "}
                {intent.data.filled
                  ? "Your intent stays as it is; the draft will wait here."
                  : "The draft will be filled in here when it is ready."}
              </span>
              <Button
                variant="ghost"
                size="xs"
                className="ml-auto"
                onClick={() => void draft.cancel()}
              >
                <CircleStopIcon />
                Cancel
              </Button>
            </Banner>
          )}
          {!draft.running && intent.data.draft && (
            <Banner tone="warn" icon={<WandSparklesIcon />}>
              This is an agent&apos;s draft, inferred from history. Fix what is
              wrong, add what only you know (the real why), then delete the line
              starting with <code>&gt; **Draft**</code> and save.
            </Banner>
          )}
          {!draft.running && keptDraft && (
            <KeptDraftBanner
              slug={slug}
              job={keptDraft}
              onLoad={(content) => {
                setReplacement({ content, nonce: Date.now() });
                toast.info(
                  "Draft loaded as unsaved changes. Revert brings your text back.",
                );
              }}
              onDismiss={() => dismiss(keptDraft.id)}
            />
          )}
          {!draft.running && failedDraft && (
            <Banner tone="crit" icon={<TriangleAlertIcon />}>
              <span className="min-w-0 truncate">
                Last draft failed: {failedDraft.error}
              </span>
              <DismissButton
                className="ml-auto"
                onClick={() => dismiss(failedDraft.id)}
              />
            </Banner>
          )}
        </>
      }
    />
  );
}

function IntentBadge({ intent }: { intent: Intent }) {
  if (intent.draft) return <Badge variant="warn">draft</Badge>;
  if (intent.filled) return <Badge variant="ok">filled</Badge>;
  return <Badge variant="warn">empty</Badge>;
}

function saveMessage(saved: Intent): string {
  if (saved.draft)
    return "Saved — still marked as a draft until you delete the Draft line";
  if (saved.filled) return "Intent saved";
  return "Saved — fill in at least Why or Goal so drift reviews have something to compare against";
}

/** A draft produced while the owner's intent already existed. */
function KeptDraftBanner({
  slug,
  job,
  onLoad,
  onDismiss,
}: {
  slug: string;
  job: Job;
  onLoad: (content: string) => void;
  onDismiss: () => void;
}) {
  const report = useQuery(`review:${slug}:${job.id}`, () =>
    api.review(slug, job.id),
  );
  const content = report.data?.report ?? "";
  return (
    <Banner tone="info" icon={<WandSparklesIcon />}>
      <span>
        A draft from {job.reviewer} ({ago(job.finished_at ?? job.created_at)}{" "}
        ago) is ready. Your intent was kept.
      </span>
      <Button
        size="xs"
        variant="outline"
        className="ml-auto"
        disabled={!content}
        onClick={() => onLoad(content)}
      >
        Load into editor
      </Button>
      <DismissButton onClick={onDismiss} />
    </Banner>
  );
}

const toneClass = {
  info: "border-info/30 bg-info/5 text-info",
  warn: "border-warn/30 bg-warn/5 text-warn",
  crit: "border-crit/30 bg-crit/5 text-crit",
};

function Banner({
  tone,
  icon,
  children,
}: {
  tone: keyof typeof toneClass;
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div
      className={cn(
        "flex shrink-0 items-center gap-2 border-b px-3 py-1.5 text-[12.5px] [&_svg]:size-3.5 [&>svg]:shrink-0",
        toneClass[tone],
      )}
    >
      {icon}
      {children}
    </div>
  );
}

function DismissButton({
  onClick,
  className,
}: {
  onClick: () => void;
  className?: string;
}) {
  return (
    <Button
      variant="ghost"
      size="icon-xs"
      className={cn("shrink-0", className)}
      title="Dismiss"
      onClick={onClick}
    >
      <XIcon />
    </Button>
  );
}
