import { useState } from "react";
import { CodeIcon, EyeIcon, FileTextIcon, LockIcon } from "lucide-react";

import { ErrorBlock, LoadingBlock } from "@/components/common";
import { Markdown } from "@/components/markdown";
import { Badge } from "@/components/ui/badge";
import { ApiError, api } from "@/lib/api";
import { t } from "@/lib/i18n";
import { useQuery } from "@/lib/query";
import { cn } from "@/lib/utils";

const markdownPattern = /\.(md|markdown|mdx)$/i;

/** Read-only view of one project file; Markdown renders, everything else is shown as text. */
export function FileViewer({
  slug,
  path,
  actions,
}: {
  slug: string;
  path: string;
  actions?: React.ReactNode;
}) {
  const file = useQuery(`file:${slug}:${path}`, () => api.file(slug, path));
  const isMarkdown = markdownPattern.test(path);
  const [raw, setRaw] = useState(false);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex h-9 shrink-0 items-center gap-2 border-b px-3">
        <FileTextIcon className="size-3.5 shrink-0 text-muted-foreground" />
        <span className="truncate font-mono text-[12px]">{path}</span>
        {file.data && <Badge variant="muted">{file.data.kind}</Badge>}
        <div className="ml-auto flex items-center gap-2">
          {actions}
          {isMarkdown && (
            <button
              type="button"
              onClick={() => setRaw(!raw)}
              title={raw ? t("Rendered") : t("Source")}
              className="flex size-6 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground"
            >
              {raw ? (
                <EyeIcon className="size-3.5" />
              ) : (
                <CodeIcon className="size-3.5" />
              )}
            </button>
          )}
        </div>
      </div>
      <div className="min-h-0 flex-1 overflow-auto">
        {file.error && <FileError error={file.error} />}
        {!file.data && !file.error && <LoadingBlock />}
        {file.data &&
          (isMarkdown && !raw ? (
            <div className="mx-auto max-w-4xl p-5">
              <Markdown>{file.data.content}</Markdown>
            </div>
          ) : (
            <pre
              className={cn(
                "p-4 font-mono text-[12px] leading-relaxed whitespace-pre",
                isMarkdown && "whitespace-pre-wrap",
              )}
            >
              {file.data.content}
            </pre>
          ))}
      </div>
    </div>
  );
}

function FileError({ error }: { error: Error }) {
  if (error instanceof ApiError && error.status === 403) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2 p-8 text-center text-xs text-muted-foreground">
        <LockIcon className="size-6" />
        <p>{error.message}</p>
        <p>
          {t(
            "xplan never serves secrets (.env, keys, certificates) to the browser.",
          )}
        </p>
      </div>
    );
  }
  if (error instanceof ApiError && error.status === 415) {
    return (
      <div className="p-8 text-center text-xs text-muted-foreground">
        {t("Binary or oversized file — not shown.")}
      </div>
    );
  }
  return <ErrorBlock error={error} />;
}
