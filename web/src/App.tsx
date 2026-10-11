import { useEffect } from "react";
import { Toaster } from "sonner";

import { ActivityBar } from "@/components/activity-bar";
import { AddProjectDialog } from "@/components/add-project-dialog";
import { CommandPalette } from "@/components/command-palette";
import { ConsolePanel } from "@/components/console-panel";
import { ParkDialog } from "@/components/park-dialog";
import { ProjectSidebar } from "@/components/project-sidebar";
import { SplitHandle, useResizable } from "@/components/splitter";
import { StatusBar } from "@/components/status-bar";
import { TitleBar } from "@/components/title-bar";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useRoute } from "@/lib/route";
import { UIProvider, useUI } from "@/lib/ui-state";
import { BoardView } from "@/views/board-view";
import { HomeView } from "@/views/home-view";
import { InboxView } from "@/views/inbox-view";
import { ProjectView } from "@/views/project/project-view";
import { SettingsView } from "@/views/settings-view";

export default function App() {
  return (
    <UIProvider>
      <TooltipProvider delay={400}>
        <Shell />
        <Toaster
          theme="dark"
          position="bottom-right"
          richColors
          closeButton
          toastOptions={{ className: "text-[13px]" }}
        />
      </TooltipProvider>
    </UIProvider>
  );
}

function Shell() {
  const [route] = useRoute();
  const ui = useUI();
  const sidebar = useResizable("xplan.sidebar", 264, 200, 440, "x");
  const consolePane = useResizable(
    "xplan.consoleHeight",
    240,
    120,
    640,
    "y",
    true,
  );
  const projectSlug = route.view === "project" ? route.slug : null;

  useGlobalShortcuts();

  return (
    <div className="flex h-full flex-col bg-background text-foreground">
      <TitleBar route={route} />
      <div className="flex min-h-0 flex-1">
        <ActivityBar route={route} />
        <aside
          style={{ width: sidebar.size }}
          className="flex min-h-0 shrink-0 flex-col bg-sidebar"
        >
          <ProjectSidebar activeSlug={projectSlug} />
        </aside>
        <SplitHandle axis="x" onPointerDown={sidebar.onPointerDown} />
        <main className="flex min-w-0 flex-1 flex-col">
          <div className="min-h-0 flex-1 overflow-hidden">
            <RouteView />
          </div>
          {ui.consoleOpen && (
            <>
              <SplitHandle axis="y" onPointerDown={consolePane.onPointerDown} />
              <div style={{ height: consolePane.size }} className="shrink-0">
                <ConsolePanel slug={projectSlug} />
              </div>
            </>
          )}
        </main>
      </div>
      <StatusBar />
      <CommandPalette />
      <AddProjectDialog />
      <ParkDialog />
    </div>
  );
}

function RouteView() {
  const [route] = useRoute();
  switch (route.view) {
    case "project":
      return (
        <ProjectView
          key={route.slug}
          slug={route.slug}
          tab={route.tab}
          arg={route.arg}
        />
      );
    case "board":
      return <BoardView />;
    case "inbox":
      return <InboxView />;
    case "settings":
      return <SettingsView />;
    default:
      return <HomeView />;
  }
}

/** ⌘K palette, ⌘J console. */
function useGlobalShortcuts() {
  const ui = useUI();
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (!(event.metaKey || event.ctrlKey)) return;
      const key = event.key.toLowerCase();
      if (key === "k") {
        event.preventDefault();
        ui.setPaletteOpen(!ui.paletteOpen);
      } else if (key === "j") {
        event.preventDefault();
        ui.toggleConsole();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [ui]);
}
