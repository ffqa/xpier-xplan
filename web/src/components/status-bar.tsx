import {
  BotIcon,
  CircleAlertIcon,
  FolderIcon,
  TriangleAlertIcon,
} from "lucide-react";

import { useMeta, useOverview } from "@/lib/ui-state";

export function StatusBar() {
  const meta = useMeta();
  const overview = useOverview();
  const cards = overview.data?.cards ?? [];
  const critical = cards.filter((card) => card.health_level === "crit").length;
  const warning = cards.filter((card) => card.health_level === "warn").length;

  return (
    <footer className="flex h-6 shrink-0 items-center gap-4 border-t bg-sidebar px-3 text-[11px] text-muted-foreground">
      <span className="flex items-center gap-1" title="xplan data directory">
        <FolderIcon className="size-3" />
        {meta.data?.home ?? "…"}
      </span>
      <span>{cards.length} projects</span>
      {critical > 0 && (
        <span className="flex items-center gap-1 text-crit">
          <CircleAlertIcon className="size-3" />
          {critical} critical
        </span>
      )}
      {warning > 0 && (
        <span className="flex items-center gap-1 text-warn">
          <TriangleAlertIcon className="size-3" />
          {warning} need attention
        </span>
      )}
      {overview.error && (
        <span className="text-crit">
          API unreachable: {overview.error.message}
        </span>
      )}
      <span
        className="ml-auto flex items-center gap-1"
        title="Default reviewer (read-only)"
      >
        <BotIcon className="size-3" />
        {meta.data?.default_reviewer ?? "…"}
      </span>
      <span>reports in {meta.data?.report_language ?? "…"}</span>
      <span>v{meta.data?.version ?? "…"}</span>
    </footer>
  );
}
