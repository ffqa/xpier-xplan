import { useCallback, useEffect, useState } from "react";

/** Hash routes: #/p/<slug>/<tab>, #/board, #/inbox, #/settings, #/ */
export type Route =
  | { view: "home" }
  | { view: "board" }
  | { view: "inbox" }
  | { view: "settings" }
  | { view: "project"; slug: string; tab: ProjectTab; arg?: string };

export const PROJECT_TABS = [
  "brief",
  "intent",
  "decisions",
  "docs",
  "reviews",
  "journal",
  "settings",
] as const;
export type ProjectTab = (typeof PROJECT_TABS)[number];

export function parseRoute(hash: string): Route {
  const parts = hash
    .replace(/^#\/?/, "")
    .split("/")
    .filter(Boolean)
    .map(decodeURIComponent);
  if (parts[0] === "p" && parts[1]) {
    const tab = (PROJECT_TABS as readonly string[]).includes(parts[2])
      ? (parts[2] as ProjectTab)
      : "brief";
    return {
      view: "project",
      slug: parts[1],
      tab,
      arg: parts.slice(3).join("/") || undefined,
    };
  }
  if (parts[0] === "board" || parts[0] === "inbox" || parts[0] === "settings")
    return { view: parts[0] };
  return { view: "home" };
}

export function routeHref(route: Route): string {
  switch (route.view) {
    case "project": {
      const base = `#/p/${encodeURIComponent(route.slug)}/${route.tab}`;
      return route.arg
        ? `${base}/${route.arg.split("/").map(encodeURIComponent).join("/")}`
        : base;
    }
    case "home":
      return "#/";
    default:
      return `#/${route.view}`;
  }
}

export function navigate(route: Route) {
  const href = routeHref(route);
  if (window.location.hash !== href) window.location.hash = href;
}

export function useRoute(): [Route, (route: Route) => void] {
  const [route, setRoute] = useState(() => parseRoute(window.location.hash));
  useEffect(() => {
    const onChange = () => setRoute(parseRoute(window.location.hash));
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  return [route, useCallback(navigate, [])];
}
