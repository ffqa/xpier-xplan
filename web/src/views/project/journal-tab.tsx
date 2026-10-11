import { useState } from "react";
import {
  BotIcon,
  Loader2Icon,
  NotebookPenIcon,
  SquareParkingIcon,
  StickyNoteIcon,
  TerminalSquareIcon,
} from "lucide-react";
import { toast } from "sonner";

import { ErrorBlock, LoadingBlock, Section } from "@/components/common";
import { ParkForm } from "@/components/park-dialog";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";
import { ago, dateTime } from "@/lib/format";
import { invalidate, useQuery } from "@/lib/query";
import type { Entry } from "@/lib/types";
import { cn } from "@/lib/utils";

const kindIcon = {
  park: SquareParkingIcon,
  note: StickyNoteIcon,
  command: TerminalSquareIcon,
  system: BotIcon,
};

const filters = ["all", "park", "note", "command", "system"] as const;
type Filter = (typeof filters)[number];

/** Park notes, free notes and console actions, newest first. */
export function JournalTab({ slug }: { slug: string }) {
  const journal = useQuery(`journal:${slug}`, () => api.journal(slug));
  const [filter, setFilter] = useState<Filter>("all");
  const entries = (journal.data?.entries ?? []).filter(
    (entry) => filter === "all" || entry.kind === filter,
  );

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto grid max-w-6xl gap-4 p-4 lg:grid-cols-[360px_minmax(0,1fr)]">
        <div className="space-y-4">
          <Section title="Park" icon={<SquareParkingIcon />}>
            <ParkForm slug={slug} compact />
          </Section>
          <NoteForm slug={slug} />
        </div>
        <Section
          title="Timeline"
          icon={<NotebookPenIcon />}
          actions={
            <div className="flex rounded-md border p-0.5">
              {filters.map((item) => (
                <button
                  key={item}
                  type="button"
                  onClick={() => setFilter(item)}
                  className={cn(
                    "rounded px-2 py-0.5 text-[11px] capitalize text-muted-foreground hover:text-foreground",
                    filter === item && "bg-accent text-foreground",
                  )}
                >
                  {item}
                </button>
              ))}
            </div>
          }
        >
          {journal.error && !journal.data && (
            <ErrorBlock error={journal.error} />
          )}
          {!journal.data && !journal.error && <LoadingBlock />}
          {journal.data && entries.length === 0 && (
            <p className="text-xs text-muted-foreground">Nothing here yet.</p>
          )}
          <ol className="relative space-y-3 before:absolute before:top-1 before:bottom-1 before:left-[9px] before:w-px before:bg-border">
            {entries.map((entry) => (
              <TimelineEntry key={entry.id} entry={entry} />
            ))}
          </ol>
        </Section>
      </div>
    </div>
  );
}

function TimelineEntry({ entry }: { entry: Entry }) {
  const Icon = kindIcon[entry.kind] ?? StickyNoteIcon;
  return (
    <li className="relative flex gap-3">
      <span
        className={cn(
          "relative z-10 flex size-5 shrink-0 items-center justify-center rounded-full border bg-card",
          entry.kind === "park" && "border-primary/50 text-primary",
        )}
      >
        <Icon className="size-3" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline gap-2 text-[11px] text-muted-foreground">
          <span className="capitalize">{entry.kind}</span>
          <span title={dateTime(entry.at)}>{ago(entry.at)} ago</span>
          {entry.head && (
            <code className="font-mono">{entry.head.slice(0, 8)}</code>
          )}
        </div>
        {entry.kind === "park" ? (
          <dl className="mt-0.5 space-y-0.5 text-[13px]">
            {entry.stopped && (
              <ParkField label="Stopped" value={entry.stopped} />
            )}
            {entry.next && <ParkField label="Next" value={entry.next} />}
            {entry.open && <ParkField label="Open" value={entry.open} />}
          </dl>
        ) : (
          <p className="mt-0.5 text-[13px] whitespace-pre-wrap">{entry.text}</p>
        )}
      </div>
    </li>
  );
}

function ParkField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex gap-2">
      <dt className="w-14 shrink-0 text-xs text-muted-foreground">{label}</dt>
      <dd className="min-w-0 whitespace-pre-wrap">{value}</dd>
    </div>
  );
}

function NoteForm({ slug }: { slug: string }) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const save = async () => {
    if (!text.trim()) return;
    setBusy(true);
    try {
      await api.addJournal(slug, { kind: "note", text: text.trim() });
      setText("");
      invalidate(`journal:${slug}`);
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <Section title="Quick note" icon={<StickyNoteIcon />}>
      <Textarea
        value={text}
        onChange={(event) => setText(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
            event.preventDefault();
            void save();
          }
        }}
        placeholder="An idea, a gotcha, a link — anything you would otherwise forget."
        rows={3}
      />
      <div className="mt-2 flex justify-end">
        <Button
          size="sm"
          variant="secondary"
          disabled={busy || !text.trim()}
          onClick={() => void save()}
        >
          {busy && <Loader2Icon className="animate-spin" />}
          Add note
        </Button>
      </div>
    </Section>
  );
}
