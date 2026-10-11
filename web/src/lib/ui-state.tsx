import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
} from "react";

import { api } from "@/lib/api";
import { invalidate, useQuery } from "@/lib/query";

/** Global UI toggles shared by the shell, the palette and the tabs. */
interface UIState {
  consoleOpen: boolean;
  toggleConsole: (open?: boolean) => void;
  paletteOpen: boolean;
  setPaletteOpen: (open: boolean) => void;
  addOpen: boolean;
  setAddOpen: (open: boolean) => void;
  parkSlug: string | null;
  openPark: (slug: string | null) => void;
  /** Text pushed into the console input; nonce changes on every push. */
  consoleSeed: { text: string; nonce: number };
  seedConsole: (text: string) => void;
}

const UIContext = createContext<UIState | null>(null);

export function UIProvider({ children }: { children: React.ReactNode }) {
  const [consoleOpen, setConsoleOpen] = useState(
    () => localStorage.getItem("xplan.console") !== "0",
  );
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [addOpen, setAddOpen] = useState(false);
  const [parkSlug, setParkSlug] = useState<string | null>(null);
  const [consoleSeed, setConsoleSeed] = useState({ text: "", nonce: 0 });

  const toggleConsole = useCallback((open?: boolean) => {
    setConsoleOpen((current) => {
      const next = open ?? !current;
      localStorage.setItem("xplan.console", next ? "1" : "0");
      return next;
    });
  }, []);

  const seedConsole = useCallback(
    (text: string) => {
      setConsoleSeed((current) => ({ text, nonce: current.nonce + 1 }));
      toggleConsole(true);
    },
    [toggleConsole],
  );

  const value = useMemo(
    () => ({
      consoleOpen,
      toggleConsole,
      paletteOpen,
      setPaletteOpen,
      addOpen,
      setAddOpen,
      parkSlug,
      openPark: setParkSlug,
      consoleSeed,
      seedConsole,
    }),
    [
      consoleOpen,
      toggleConsole,
      paletteOpen,
      addOpen,
      parkSlug,
      consoleSeed,
      seedConsole,
    ],
  );
  return <UIContext.Provider value={value}>{children}</UIContext.Provider>;
}

export function useUI(): UIState {
  const value = useContext(UIContext);
  if (!value) throw new Error("useUI must be used inside UIProvider");
  return value;
}

/** Shared queries used by several components. */
export function useMeta() {
  return useQuery("meta", api.meta);
}

export function useOverview() {
  return useQuery("overview", api.overview, { refetchInterval: 15000 });
}

/** Refresh everything that depends on a project's state. */
export function refreshProject(slug: string) {
  invalidate(
    "overview",
    `brief:${slug}`,
    `docs:${slug}`,
    `reviews:${slug}`,
    `journal:${slug}`,
    `adrs:${slug}`,
    `intent:${slug}`,
    `project:${slug}`,
  );
}
