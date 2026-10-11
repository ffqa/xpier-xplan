import { useState } from "react";
import { TargetIcon } from "lucide-react";
import { toast } from "sonner";

import { ErrorBlock, LoadingBlock } from "@/components/common";
import { MarkdownEditor } from "@/components/markdown-editor";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";
import { setQueryData, useQuery } from "@/lib/query";
import { refreshProject } from "@/lib/ui-state";

/** intent.md: why the project exists. Owned by xplan, never by the project. */
export function IntentTab({ slug }: { slug: string }) {
  const intent = useQuery(`intent:${slug}`, () => api.intent(slug));
  const [saving, setSaving] = useState(false);

  const save = async (content: string) => {
    setSaving(true);
    try {
      const saved = await api.saveIntent(slug, content);
      setQueryData(`intent:${slug}`, () => saved);
      toast.success(
        saved.filled
          ? "Intent saved"
          : "Saved — fill in at least Why or Goal so drift reviews have something to compare against",
      );
      refreshProject(slug);
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setSaving(false);
    }
  };

  if (intent.error && !intent.data) return <ErrorBlock error={intent.error} />;
  if (!intent.data) return <LoadingBlock />;

  return (
    <MarkdownEditor
      storageKey="intent"
      value={intent.data.content}
      saving={saving}
      onSave={save}
      toolbarLeft={
        <>
          <TargetIcon className="size-3.5 text-muted-foreground" />
          <span className="text-[12.5px] font-medium">intent.md</span>
          {intent.data.filled ? (
            <Badge variant="ok">filled</Badge>
          ) : (
            <Badge variant="warn">empty</Badge>
          )}
          <span className="hidden text-[11px] text-muted-foreground lg:inline">
            Why and Goal are what drift reviews compare the code against. Write
            them in your own words.
          </span>
        </>
      }
    />
  );
}
