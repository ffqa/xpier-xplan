import {
  FolderTreeIcon,
  InboxIcon,
  KanbanSquareIcon,
  Settings2Icon,
  TerminalSquareIcon,
  type LucideIcon,
} from "lucide-react";

import { Tip } from "@/components/ui/tooltip";
import { api } from "@/lib/api";
import { t } from "@/lib/i18n";
import { useQuery } from "@/lib/query";
import { routeHref, type Route } from "@/lib/route";
import { useUI } from "@/lib/ui-state";
import { cn } from "@/lib/utils";

interface ActivityItem {
  view: "home" | "board" | "inbox" | "settings";
  label: string;
  icon: LucideIcon;
}

const items: ActivityItem[] = [
  {
    view: "home",
    label: "Overview — where every project stands",
    icon: FolderTreeIcon,
  },
  {
    view: "board",
    label: "Board — drag projects between states",
    icon: KanbanSquareIcon,
  },
  {
    view: "inbox",
    label: "Inbox — findings from the latest reviews",
    icon: InboxIcon,
  },
];

export function ActivityBar({ route }: { route: Route }) {
  const ui = useUI();
  const inbox = useQuery("inbox:high", () => api.inbox("high"), {
    refetchInterval: 60000,
  });
  const urgent = inbox.data?.items.length ?? 0;

  return (
    <nav className="flex w-12 shrink-0 flex-col items-center gap-1 border-r bg-sidebar py-2">
      {items.map((item) => (
        <ActivityLink
          key={item.view}
          item={item}
          active={
            route.view === item.view ||
            (item.view === "home" && route.view === "project")
          }
          badge={item.view === "inbox" ? urgent : 0}
        />
      ))}
      <div className="mt-auto flex flex-col items-center gap-1">
        <Tip label={t("Console (⌘J)")} side="right">
          <button
            type="button"
            onClick={() => ui.toggleConsole()}
            className={cn(
              "flex size-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:text-foreground",
              ui.consoleOpen && "text-foreground",
            )}
          >
            <TerminalSquareIcon className="size-[18px]" />
          </button>
        </Tip>
        <ActivityLink
          item={{ view: "settings", label: "Settings", icon: Settings2Icon }}
          active={route.view === "settings"}
          badge={0}
        />
      </div>
    </nav>
  );
}

function ActivityLink({
  item,
  active,
  badge,
}: {
  item: ActivityItem;
  active: boolean;
  badge: number;
}) {
  const Icon = item.icon;
  return (
    <Tip label={t(item.label)} side="right">
      <a
        href={routeHref({ view: item.view } as Route)}
        className={cn(
          "relative flex size-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:text-foreground",
          active &&
            "text-foreground before:absolute before:top-1.5 before:bottom-1.5 before:-left-1.5 before:w-0.5 before:rounded-full before:bg-primary",
        )}
      >
        <Icon className="size-[18px]" />
        {badge > 0 && (
          <span className="absolute top-0.5 right-0.5 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-crit px-1 text-[9px] font-semibold text-white">
            {badge > 99 ? "99+" : badge}
          </span>
        )}
      </a>
    </Tip>
  );
}
