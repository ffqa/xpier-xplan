import { useEffect, useState } from "react";
import {
  ColumnsIcon,
  EyeIcon,
  Loader2Icon,
  PencilIcon,
  SaveIcon,
} from "lucide-react";

import { Markdown } from "@/components/markdown";
import { Button } from "@/components/ui/button";
import { Kbd } from "@/components/ui/misc";
import { cn } from "@/lib/utils";

type Mode = "edit" | "split" | "preview";

/**
 * Markdown editor with live preview and ⌘S to save. Used for intent and ADRs.
 * The parent owns persistence; this component only tracks the draft.
 */
export function MarkdownEditor({
  value,
  onSave,
  saving,
  toolbarLeft,
  banner,
  replacement,
  storageKey,
}: {
  value: string;
  onSave: (content: string) => Promise<void> | void;
  saving?: boolean;
  toolbarLeft?: React.ReactNode;
  /** Shown between the toolbar and the editor. */
  banner?: React.ReactNode;
  /** Loads text into the editor as unsaved changes; bump nonce to reapply. */
  replacement?: { content: string; nonce: number };
  storageKey: string;
}) {
  const [draft, setDraft] = useState(value);
  const [syncedValue, setSyncedValue] = useState(value);
  const [mode, setMode] = useState<Mode>(
    () =>
      (localStorage.getItem(`xplan.editor.${storageKey}`) as Mode) || "split",
  );
  const dirty = draft !== value;

  // A server-side change (e.g. an agent's draft landing) must not wipe unsaved typing.
  if (value !== syncedValue) {
    setSyncedValue(value);
    if (draft === syncedValue) setDraft(value);
  }

  useEffect(() => {
    if (replacement) setDraft(replacement.content);
  }, [replacement]);

  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);

  const changeMode = (next: Mode) => {
    setMode(next);
    localStorage.setItem(`xplan.editor.${storageKey}`, next);
  };

  const onKeyDown = (event: React.KeyboardEvent) => {
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s") {
      event.preventDefault();
      if (dirty) void onSave(draft);
    }
  };

  return (
    <div className="flex h-full min-h-0 flex-col" onKeyDown={onKeyDown}>
      <div className="flex h-9 shrink-0 items-center gap-2 border-b px-3">
        {toolbarLeft}
        <div className="ml-auto flex items-center gap-2">
          <span
            className={cn(
              "text-[11px]",
              dirty ? "text-warn" : "text-muted-foreground",
            )}
          >
            {dirty ? "Unsaved changes" : "Saved"}
          </span>
          <div className="flex rounded-md border p-0.5">
            <ModeButton
              active={mode === "edit"}
              onClick={() => changeMode("edit")}
              label="Edit"
            >
              <PencilIcon />
            </ModeButton>
            <ModeButton
              active={mode === "split"}
              onClick={() => changeMode("split")}
              label="Split"
            >
              <ColumnsIcon />
            </ModeButton>
            <ModeButton
              active={mode === "preview"}
              onClick={() => changeMode("preview")}
              label="Preview"
            >
              <EyeIcon />
            </ModeButton>
          </div>
          {dirty && (
            <Button variant="ghost" size="xs" onClick={() => setDraft(value)}>
              Revert
            </Button>
          )}
          <Button
            size="xs"
            disabled={!dirty || saving}
            onClick={() => void onSave(draft)}
          >
            {saving ? <Loader2Icon className="animate-spin" /> : <SaveIcon />}
            Save{" "}
            <Kbd className="h-4 border-primary-foreground/30 bg-transparent text-primary-foreground/80">
              ⌘S
            </Kbd>
          </Button>
        </div>
      </div>
      {banner}
      <div className="flex min-h-0 flex-1">
        {mode !== "preview" && (
          <textarea
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            spellCheck={false}
            className={cn(
              "h-full min-w-0 flex-1 resize-none bg-transparent p-4 font-mono text-[12.5px] leading-relaxed outline-none",
              mode === "split" && "border-r",
            )}
          />
        )}
        {mode !== "edit" && (
          <div className="h-full min-w-0 flex-1 overflow-y-auto p-5">
            <Markdown>{draft}</Markdown>
          </div>
        )}
      </div>
    </div>
  );
}

function ModeButton({
  active,
  onClick,
  label,
  children,
}: {
  active: boolean;
  onClick: () => void;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      title={label}
      onClick={onClick}
      className={cn(
        "flex size-6 items-center justify-center rounded text-muted-foreground transition-colors hover:text-foreground [&_svg]:size-3.5",
        active && "bg-accent text-foreground",
      )}
    >
      {children}
    </button>
  );
}
