import { useState } from "react";
import {
  FileTextIcon,
  LightbulbIcon,
  Loader2Icon,
  PlusIcon,
} from "lucide-react";
import { toast } from "sonner";

import { Empty, ErrorBlock, LoadingBlock } from "@/components/common";
import { FileViewer } from "@/components/file-viewer";
import { MarkdownEditor } from "@/components/markdown-editor";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { ago } from "@/lib/format";
import { invalidate, useQuery } from "@/lib/query";
import { navigate, routeHref } from "@/lib/route";
import type { ADR, Brief, Doc } from "@/lib/types";
import { cn } from "@/lib/utils";

/**
 * Decisions = xplan's own ADRs (editable, stored in ~/.xplan) plus the ADR and
 * spec documents found inside the project (read-only).
 */
export function DecisionsTab({
  slug,
  arg,
  brief,
}: {
  slug: string;
  arg?: string;
  brief: Brief;
}) {
  const adrs = useQuery(`adrs:${slug}`, () => api.adrs(slug));
  const projectDocs = (brief.snapshot?.docs ?? []).filter(
    (doc) => doc.kind === "adr" || doc.kind === "spec",
  );
  const list = adrs.data?.adrs ?? [];
  const selectedADR = list.find((adr) => adr.id === arg);
  const selectedDoc = !selectedADR
    ? projectDocs.find((doc) => doc.path === arg)
    : undefined;

  if (adrs.error && !adrs.data) return <ErrorBlock error={adrs.error} />;
  if (!adrs.data) return <LoadingBlock />;

  return (
    <div className="flex h-full min-h-0">
      <aside className="flex w-72 shrink-0 flex-col border-r">
        <NewADR slug={slug} />
        <div className="min-h-0 flex-1 overflow-y-auto pb-2">
          <ListHeader>xplan decisions · {list.length}</ListHeader>
          {list.length === 0 && (
            <p className="px-3 py-1 text-xs text-muted-foreground">
              Record why you chose something, so future-you doesn&apos;t
              re-litigate it.
            </p>
          )}
          {list.map((adr) => (
            <ADRItem
              key={adr.id}
              slug={slug}
              adr={adr}
              active={adr.id === arg}
            />
          ))}
          <ListHeader>In the project · {projectDocs.length}</ListHeader>
          {projectDocs.length === 0 && (
            <p className="px-3 py-1 text-xs text-muted-foreground">
              No ADR or spec files found in the repository.
            </p>
          )}
          {projectDocs.map((doc) => (
            <DocItem
              key={doc.path}
              slug={slug}
              doc={doc}
              active={doc.path === arg}
            />
          ))}
        </div>
      </aside>
      <div className="min-w-0 flex-1">
        {selectedADR && <ADREditor slug={slug} adr={selectedADR} />}
        {selectedDoc && <ProjectDocViewer slug={slug} doc={selectedDoc} />}
        {!selectedADR && !selectedDoc && (
          <Empty
            icon={<LightbulbIcon />}
            title="Pick a decision"
            className="h-full"
          >
            xplan decisions live outside the repository, so they survive branch
            switches and stay yours. Project ADRs and specs are shown read-only.
          </Empty>
        )}
      </div>
    </div>
  );
}

function ListHeader({ children }: { children: React.ReactNode }) {
  return (
    <div className="px-3 pt-3 pb-1 text-[10.5px] font-semibold tracking-wider text-muted-foreground uppercase">
      {children}
    </div>
  );
}

function NewADR({ slug }: { slug: string }) {
  const [title, setTitle] = useState("");
  const [busy, setBusy] = useState(false);
  const create = async () => {
    if (!title.trim()) return;
    setBusy(true);
    try {
      const created = await api.createADR(slug, title.trim());
      setTitle("");
      invalidate(`adrs:${slug}`, `brief:${slug}`);
      navigate({ view: "project", slug, tab: "decisions", arg: created.id });
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <form
      className="flex gap-1.5 border-b p-2"
      onSubmit={(event) => {
        event.preventDefault();
        void create();
      }}
    >
      <Input
        value={title}
        onChange={(event) => setTitle(event.target.value)}
        placeholder="New decision title"
        className="h-7 text-[12.5px]"
      />
      <Button type="submit" size="icon-sm" disabled={busy || !title.trim()}>
        {busy ? <Loader2Icon className="animate-spin" /> : <PlusIcon />}
      </Button>
    </form>
  );
}

function ADRItem({
  slug,
  adr,
  active,
}: {
  slug: string;
  adr: ADR;
  active: boolean;
}) {
  return (
    <a
      href={routeHref({ view: "project", slug, tab: "decisions", arg: adr.id })}
      className={cn(
        "mx-1 flex items-center gap-2 rounded-md px-2 py-1.5 text-[12.5px] hover:bg-accent/50",
        active && "bg-accent",
      )}
    >
      <span className="font-mono text-[11px] text-muted-foreground">
        {String(adr.number).padStart(4, "0")}
      </span>
      <span className="min-w-0 flex-1 truncate">{adr.title}</span>
      <StatusBadge status={adr.status} />
    </a>
  );
}

function DocItem({
  slug,
  doc,
  active,
}: {
  slug: string;
  doc: Doc;
  active: boolean;
}) {
  return (
    <a
      href={routeHref({
        view: "project",
        slug,
        tab: "decisions",
        arg: doc.path,
      })}
      title={doc.path}
      className={cn(
        "mx-1 flex items-center gap-2 rounded-md px-2 py-1.5 text-[12.5px] hover:bg-accent/50",
        active && "bg-accent",
      )}
    >
      <FileTextIcon className="size-3 shrink-0 text-muted-foreground" />
      <span className="min-w-0 flex-1 truncate">{doc.title || doc.path}</span>
      <Badge variant="muted">{doc.kind}</Badge>
    </a>
  );
}

function StatusBadge({ status }: { status: string }) {
  const variant =
    status === "accepted"
      ? "ok"
      : status === "superseded" || status === "deprecated"
        ? "muted"
        : status === "rejected"
          ? "crit"
          : "info";
  return <Badge variant={variant}>{status || "—"}</Badge>;
}

function ADREditor({ slug, adr }: { slug: string; adr: ADR }) {
  const [saving, setSaving] = useState(false);
  const save = async (content: string) => {
    setSaving(true);
    try {
      await api.saveADR(slug, adr.id, content);
      toast.success("Decision saved");
      invalidate(`adrs:${slug}`, `brief:${slug}`);
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setSaving(false);
    }
  };
  return (
    <MarkdownEditor
      key={adr.id}
      storageKey="adr"
      value={adr.content}
      saving={saving}
      onSave={save}
      toolbarLeft={
        <>
          <span className="font-mono text-[11px] text-muted-foreground">
            {adr.id}.md
          </span>
          <StatusBadge status={adr.status} />
          <span className="text-[11px] text-muted-foreground">
            edited {ago(adr.updated_at)} ago · change “Status:” in the text to
            accept or supersede
          </span>
        </>
      }
    />
  );
}

function ProjectDocViewer({ slug, doc }: { slug: string; doc: Doc }) {
  return (
    <FileViewer
      slug={slug}
      path={doc.path}
      actions={
        <a
          href={routeHref({
            view: "project",
            slug,
            tab: "docs",
            arg: doc.path,
          })}
          className="text-[11px] text-primary hover:underline"
        >
          Open in Docs
        </a>
      }
    />
  );
}
