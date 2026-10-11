import { useEffect, useMemo, useRef, useState } from "react";
import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import {
  FolderIcon,
  InboxIcon,
  KanbanSquareIcon,
  LayersIcon,
  PlusIcon,
  SearchIcon,
  Settings2Icon,
  SquareParkingIcon,
  TerminalSquareIcon,
  type LucideIcon,
} from "lucide-react";
import { toast } from "sonner";

import { HealthDot, kindIcon } from "@/components/status";
import { Kbd } from "@/components/ui/misc";
import { api } from "@/lib/api";
import { invalidate } from "@/lib/query";
import { navigate, PROJECT_TABS, useRoute } from "@/lib/route";
import type { Level } from "@/lib/types";
import { useMeta, useOverview, useUI } from "@/lib/ui-state";
import { cn } from "@/lib/utils";

interface PaletteItem {
  id: string;
  group: string;
  label: string;
  hint?: string;
  icon?: LucideIcon;
  level?: Level;
  keywords?: string;
  run: () => void;
}

export function CommandPalette() {
  const ui = useUI();
  return (
    <DialogPrimitive.Root
      open={ui.paletteOpen}
      onOpenChange={ui.setPaletteOpen}
    >
      <DialogPrimitive.Portal>
        <DialogPrimitive.Backdrop className="fixed inset-0 z-50 bg-black/45 data-open:animate-in data-open:fade-in-0" />
        <DialogPrimitive.Popup className="fixed top-[12%] left-1/2 z-50 w-full max-w-xl -translate-x-1/2 overflow-hidden rounded-lg border bg-popover shadow-2xl outline-none data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95">
          {ui.paletteOpen && (
            <PaletteBody close={() => ui.setPaletteOpen(false)} />
          )}
        </DialogPrimitive.Popup>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}

function PaletteBody({ close }: { close: () => void }) {
  const items = usePaletteItems(close);
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);

  const filtered = useMemo(() => {
    const words = query.trim().toLowerCase().split(/\s+/).filter(Boolean);
    if (words.length === 0) return items;
    return items.filter((item) => {
      const haystack =
        `${item.group} ${item.label} ${item.hint ?? ""} ${item.keywords ?? ""}`.toLowerCase();
      return words.every((word) => haystack.includes(word));
    });
  }, [items, query]);

  useEffect(() => setSelected(0), [query]);
  useEffect(() => {
    listRef.current
      ?.querySelector(`[data-index="${selected}"]`)
      ?.scrollIntoView({ block: "nearest" });
  }, [selected]);

  const onKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const step = event.key === "ArrowDown" ? 1 : -1;
      setSelected((current) =>
        filtered.length
          ? (current + step + filtered.length) % filtered.length
          : 0,
      );
    } else if (event.key === "Enter" && filtered[selected]) {
      event.preventDefault();
      filtered[selected].run();
    }
  };

  let lastGroup = "";
  return (
    <div onKeyDown={onKeyDown}>
      <div className="flex h-11 items-center gap-2 border-b px-3">
        <SearchIcon className="size-4 text-muted-foreground" />
        <input
          autoFocus
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search projects, tabs and actions…"
          className="h-full flex-1 bg-transparent text-[14px] outline-none placeholder:text-muted-foreground"
        />
        <Kbd>esc</Kbd>
      </div>
      <div
        ref={listRef}
        className="max-h-[min(60vh,440px)] overflow-y-auto p-1"
      >
        {filtered.length === 0 && (
          <p className="p-6 text-center text-xs text-muted-foreground">
            Nothing matches “{query}”.
          </p>
        )}
        {filtered.map((item, index) => {
          const header = item.group !== lastGroup ? item.group : null;
          lastGroup = item.group;
          const Icon = item.icon;
          return (
            <div key={item.id}>
              {header && (
                <div className="px-2 pt-2 pb-1 text-[10.5px] font-semibold tracking-wider text-muted-foreground uppercase">
                  {header}
                </div>
              )}
              <button
                type="button"
                data-index={index}
                onMouseMove={() => setSelected(index)}
                onClick={item.run}
                className={cn(
                  "flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-[13px]",
                  index === selected && "bg-accent text-accent-foreground",
                )}
              >
                {item.level ? (
                  <HealthDot level={item.level} className="mx-0.5" />
                ) : Icon ? (
                  <Icon className="size-3.5 text-muted-foreground" />
                ) : null}
                <span className="truncate">{item.label}</span>
                {item.hint && (
                  <span className="ml-auto truncate pl-3 text-xs text-muted-foreground">
                    {item.hint}
                  </span>
                )}
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function usePaletteItems(close: () => void): PaletteItem[] {
  const ui = useUI();
  const meta = useMeta();
  const overview = useOverview();
  const [route] = useRoute();
  const current = route.view === "project" ? route.slug : null;

  return useMemo(() => {
    const go = (action: () => void) => () => {
      close();
      action();
    };
    const list: PaletteItem[] = [];

    if (current) {
      for (const tab of PROJECT_TABS) {
        list.push({
          id: `tab-${tab}`,
          group: "This project",
          label: `Go to ${tab}`,
          icon: LayersIcon,
          run: go(() => navigate({ view: "project", slug: current, tab })),
        });
      }
      list.push({
        id: "park",
        group: "This project",
        label: "Park — leave a note before switching",
        icon: SquareParkingIcon,
        run: go(() => ui.openPark(current)),
      });
      for (const kind of meta.data?.review_kinds ?? []) {
        if (kind.id === "ask") continue;
        list.push({
          id: `review-${kind.id}`,
          group: "This project",
          label: `Run ${kind.title}`,
          hint: "read-only",
          icon: kindIcon[kind.id],
          keywords: "review",
          run: go(() => {
            api
              .startReview(current, kind.id)
              .then(() => {
                toast.success(`${kind.title} queued`);
                invalidate(
                  "overview",
                  `reviews:${current}`,
                  `brief:${current}`,
                );
              })
              .catch((error: Error) => toast.error(error.message));
          }),
        });
      }
    }

    for (const card of overview.data?.cards ?? []) {
      list.push({
        id: `project-${card.slug}`,
        group: "Projects",
        label: card.name,
        hint: card.last_park?.next || card.summary,
        level: card.health_level,
        keywords: `${card.slug} ${card.status} ${(card.tags ?? []).join(" ")}`,
        run: go(() =>
          navigate({ view: "project", slug: card.slug, tab: "brief" }),
        ),
      });
    }

    list.push(
      {
        id: "view-home",
        group: "Views",
        label: "Overview",
        icon: FolderIcon,
        run: go(() => navigate({ view: "home" })),
      },
      {
        id: "view-board",
        group: "Views",
        label: "Board",
        icon: KanbanSquareIcon,
        run: go(() => navigate({ view: "board" })),
      },
      {
        id: "view-inbox",
        group: "Views",
        label: "Inbox",
        icon: InboxIcon,
        run: go(() => navigate({ view: "inbox" })),
      },
      {
        id: "view-settings",
        group: "Views",
        label: "Settings",
        icon: Settings2Icon,
        run: go(() => navigate({ view: "settings" })),
      },
      {
        id: "add",
        group: "Actions",
        label: "Register a project",
        icon: PlusIcon,
        run: go(() => ui.setAddOpen(true)),
      },
      {
        id: "console",
        group: "Actions",
        label: "Toggle console",
        hint: "⌘J",
        icon: TerminalSquareIcon,
        run: go(() => ui.toggleConsole()),
      },
    );

    if (current) {
      for (const command of meta.data?.commands ?? []) {
        list.push({
          id: `cmd-${command.name}`,
          group: "Console commands",
          label: `${command.name} ${command.args}`.trim(),
          hint: command.summary,
          icon: TerminalSquareIcon,
          run: go(() =>
            ui.seedConsole(command.name + (command.args ? " " : "")),
          ),
        });
      }
    }
    return list;
  }, [close, current, meta.data, overview.data, ui]);
}
