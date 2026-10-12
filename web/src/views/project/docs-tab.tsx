import { useMemo, useState } from "react";
import {
  BookOpenIcon,
  ChevronRightIcon,
  FileIcon,
  FileTextIcon,
  FolderIcon,
  FolderOpenIcon,
  LockIcon,
  SearchIcon,
} from "lucide-react";

import { Empty, ErrorBlock, LoadingBlock, Spinner } from "@/components/common";
import { FileViewer } from "@/components/file-viewer";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";
import { ago, fileSize } from "@/lib/format";
import { t } from "@/lib/i18n";
import { useQuery } from "@/lib/query";
import { navigate, routeHref } from "@/lib/route";
import type { Doc, TreeEntry } from "@/lib/types";
import { cn } from "@/lib/utils";

const kindLabel: Record<string, string> = {
  readme: "Readme",
  spec: "Specs & plans",
  adr: "Decision records",
  progress: "Progress & tasks",
  changelog: "Changelogs",
  agent: "Agent instructions",
  xteam: "xteam slices",
  doc: "Other docs",
};

function kindTitle(kind: string) {
  return kindLabel[kind] ? t(kindLabel[kind]) : kind;
}

type Mode = "kinds" | "files";

/** Every doc the scanner collected, plus a lazy file tree of the whole project. */
export function DocsTab({ slug, arg }: { slug: string; arg?: string }) {
  const [mode, setMode] = useState<Mode>(
    () => (localStorage.getItem("xplan.docsMode") as Mode) || "kinds",
  );
  const changeMode = (next: Mode) => {
    setMode(next);
    localStorage.setItem("xplan.docsMode", next);
  };

  return (
    <div className="flex h-full min-h-0">
      <aside className="flex w-80 shrink-0 flex-col border-r">
        <div className="flex h-9 shrink-0 items-center gap-1 border-b px-2">
          <ModeTab
            active={mode === "kinds"}
            onClick={() => changeMode("kinds")}
          >
            {t("Docs by kind")}
          </ModeTab>
          <ModeTab
            active={mode === "files"}
            onClick={() => changeMode("files")}
          >
            {t("Files")}
          </ModeTab>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {mode === "kinds" ? (
            <DocsByKind slug={slug} selected={arg} />
          ) : (
            <FileTree slug={slug} selected={arg} />
          )}
        </div>
      </aside>
      <div className="min-w-0 flex-1">
        {arg ? (
          <FileViewer key={arg} slug={slug} path={arg} />
        ) : (
          <Empty
            icon={<BookOpenIcon />}
            title={t("Pick a document")}
            className="h-full"
          >
            {t(
              "Docs are collected from the project on every scan: READMEs, specs, ADRs, progress files, changelogs, agent rules and xteam slices.",
            )}
          </Empty>
        )}
      </div>
    </div>
  );
}

function ModeTab({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded px-2 py-1 text-[12px] text-muted-foreground transition-colors hover:text-foreground",
        active && "bg-accent text-foreground",
      )}
    >
      {children}
    </button>
  );
}

function DocsByKind({ slug, selected }: { slug: string; selected?: string }) {
  const docs = useQuery(`docs:${slug}`, () => api.docs(slug));
  const [filter, setFilter] = useState("");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

  const groups = useMemo(() => {
    const needle = filter.trim().toLowerCase();
    const list = (docs.data?.docs ?? []).filter(
      (doc) =>
        !needle ||
        doc.path.toLowerCase().includes(needle) ||
        doc.title.toLowerCase().includes(needle),
    );
    return (docs.data?.kinds ?? Object.keys(kindLabel))
      .map((kind) => ({
        kind,
        docs: list
          .filter((doc) => doc.kind === kind)
          .sort((a, b) => b.mod_time.localeCompare(a.mod_time)),
      }))
      .filter((group) => group.docs.length > 0);
  }, [docs.data, filter]);

  if (docs.error && !docs.data) return <ErrorBlock error={docs.error} />;
  if (!docs.data) return <LoadingBlock />;

  return (
    <div className="pb-3">
      <div className="p-2">
        <label className="flex h-7 items-center gap-1.5 rounded-md border bg-background/50 px-2 focus-within:border-ring/60">
          <SearchIcon className="size-3.5 text-muted-foreground" />
          <input
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder={t("Filter docs")}
            className="h-full w-full bg-transparent text-[12.5px] outline-none placeholder:text-muted-foreground"
          />
        </label>
      </div>
      {groups.length === 0 && (
        <p className="px-3 text-xs text-muted-foreground">
          {t("No docs found.")}
        </p>
      )}
      {groups.map((group) => (
        <div key={group.kind} className="mb-1">
          <button
            type="button"
            onClick={() =>
              setCollapsed({
                ...collapsed,
                [group.kind]: !collapsed[group.kind],
              })
            }
            className="flex h-6 w-full items-center gap-1 px-2 text-[11px] font-medium tracking-wide text-muted-foreground uppercase hover:text-foreground"
          >
            <ChevronRightIcon
              className={cn(
                "size-3 transition-transform",
                !collapsed[group.kind] && "rotate-90",
              )}
            />
            {kindTitle(group.kind)}
            <span className="ml-auto font-normal tabular-nums">
              {group.docs.length}
            </span>
          </button>
          {!collapsed[group.kind] &&
            group.docs.map((doc) => (
              <DocRow
                key={doc.path}
                slug={slug}
                doc={doc}
                active={doc.path === selected}
              />
            ))}
        </div>
      ))}
      {docs.data.truncated && (
        <p className="px-3 pt-2 text-[11px] text-warn">
          {t(
            "Doc list truncated — the project has more docs than the scanner keeps.",
          )}
        </p>
      )}
    </div>
  );
}

function DocRow({
  slug,
  doc,
  active,
}: {
  slug: string;
  doc: Doc;
  active: boolean;
}) {
  const total = doc.checked + doc.unchecked;
  return (
    <a
      href={routeHref({ view: "project", slug, tab: "docs", arg: doc.path })}
      title={doc.path}
      className={cn(
        "mx-1 flex flex-col rounded-md px-2 py-1 hover:bg-accent/50",
        active && "bg-accent",
      )}
    >
      <span className="flex items-center gap-2">
        <FileTextIcon className="size-3 shrink-0 text-muted-foreground" />
        <span className="min-w-0 flex-1 truncate text-[12.5px]">
          {doc.title || doc.path.split("/").pop()}
        </span>
        {total > 0 && (
          <Badge
            variant={doc.unchecked === 0 ? "ok" : "muted"}
            className="tabular-nums"
          >
            {doc.checked}/{total}
          </Badge>
        )}
      </span>
      <span className="flex gap-2 pl-5 text-[11px] text-muted-foreground">
        <span className="min-w-0 flex-1 truncate font-mono">{doc.path}</span>
        <span className="shrink-0">{ago(doc.mod_time)}</span>
      </span>
    </a>
  );
}

function FileTree({ slug, selected }: { slug: string; selected?: string }) {
  return (
    <div className="py-1">
      <TreeLevel slug={slug} path="" depth={0} selected={selected} />
    </div>
  );
}

function TreeLevel({
  slug,
  path,
  depth,
  selected,
}: {
  slug: string;
  path: string;
  depth: number;
  selected?: string;
}) {
  const tree = useQuery(`tree:${slug}:${path}`, () => api.tree(slug, path));
  if (tree.error && !tree.data)
    return (
      <p className="px-3 py-1 text-[11px] text-crit">{tree.error.message}</p>
    );
  if (!tree.data) {
    return (
      <div style={{ paddingLeft: 12 + depth * 12 }} className="py-1">
        <Spinner className="size-3" />
      </div>
    );
  }
  return (
    <>
      {tree.data.entries.map((entry) => (
        <TreeNode
          key={entry.path}
          slug={slug}
          entry={entry}
          depth={depth}
          selected={selected}
        />
      ))}
    </>
  );
}

function TreeNode({
  slug,
  entry,
  depth,
  selected,
}: {
  slug: string;
  entry: TreeEntry;
  depth: number;
  selected?: string;
}) {
  const [open, setOpen] = useState(
    () => !!selected?.startsWith(entry.path + "/"),
  );
  const indent = { paddingLeft: 8 + depth * 12 };

  if (entry.is_dir) {
    return (
      <>
        <button
          type="button"
          onClick={() => setOpen(!open)}
          style={indent}
          className="flex h-6 w-full items-center gap-1 pr-2 text-left text-[12.5px] hover:bg-accent/50"
        >
          <ChevronRightIcon
            className={cn(
              "size-3 shrink-0 text-muted-foreground transition-transform",
              open && "rotate-90",
            )}
          />
          {open ? (
            <FolderOpenIcon className="size-3.5 shrink-0 text-info" />
          ) : (
            <FolderIcon className="size-3.5 shrink-0 text-info" />
          )}
          <span className="truncate">{entry.name}</span>
        </button>
        {open && (
          <TreeLevel
            slug={slug}
            path={entry.path}
            depth={depth + 1}
            selected={selected}
          />
        )}
      </>
    );
  }
  return (
    <button
      type="button"
      disabled={entry.sensitive}
      onClick={() =>
        navigate({ view: "project", slug, tab: "docs", arg: entry.path })
      }
      style={{ paddingLeft: 8 + depth * 12 + 16 }}
      title={entry.sensitive ? t("Sensitive file — never shown") : entry.path}
      className={cn(
        "flex h-6 w-full items-center gap-1.5 pr-2 text-left text-[12.5px] hover:bg-accent/50 disabled:cursor-not-allowed disabled:opacity-50",
        selected === entry.path && "bg-accent",
      )}
    >
      {entry.sensitive ? (
        <LockIcon className="size-3.5 shrink-0 text-warn" />
      ) : (
        <FileIcon className="size-3.5 shrink-0 text-muted-foreground" />
      )}
      <span className="min-w-0 flex-1 truncate">{entry.name}</span>
      <span className="shrink-0 text-[10.5px] text-muted-foreground/70">
        {fileSize(entry.size)}
      </span>
    </button>
  );
}
