import { useCallback, useEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";

/**
 * Persisted pane size (px) with a drag handle, like an IDE splitter.
 * axis "x" resizes width (handle on the right edge of the pane),
 * axis "y" resizes height (handle on the top edge of a bottom panel).
 */
export function useResizable(
  storageKey: string,
  initial: number,
  min: number,
  max: number,
  axis: "x" | "y",
  invert = false,
) {
  const [size, setSize] = useState(() => {
    const stored = Number(localStorage.getItem(storageKey));
    return Number.isFinite(stored) && stored >= min && stored <= max
      ? stored
      : initial;
  });
  const drag = useRef<{ start: number; startSize: number } | null>(null);

  useEffect(() => {
    localStorage.setItem(storageKey, String(Math.round(size)));
  }, [storageKey, size]);

  const onPointerDown = useCallback(
    (event: React.PointerEvent) => {
      event.preventDefault();
      drag.current = {
        start: axis === "x" ? event.clientX : event.clientY,
        startSize: size,
      };
      const onMove = (move: PointerEvent) => {
        if (!drag.current) return;
        const delta =
          (axis === "x" ? move.clientX : move.clientY) - drag.current.start;
        const next = drag.current.startSize + (invert ? -delta : delta);
        setSize(Math.max(min, Math.min(max, next)));
      };
      const onUp = () => {
        drag.current = null;
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("pointerup", onUp);
      };
      document.body.style.cursor = axis === "x" ? "col-resize" : "row-resize";
      document.body.style.userSelect = "none";
      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onUp);
    },
    [axis, invert, max, min, size],
  );

  return { size, onPointerDown };
}

export function SplitHandle({
  axis,
  onPointerDown,
  className,
}: {
  axis: "x" | "y";
  onPointerDown: (event: React.PointerEvent) => void;
  className?: string;
}) {
  return (
    <div
      role="separator"
      aria-orientation={axis === "x" ? "vertical" : "horizontal"}
      onPointerDown={onPointerDown}
      className={cn(
        "group relative z-10 shrink-0 bg-border transition-colors hover:bg-primary/60",
        axis === "x" ? "w-px cursor-col-resize" : "h-px cursor-row-resize",
        className,
      )}
    >
      <div
        className={cn(
          "absolute",
          axis === "x" ? "inset-y-0 -left-1 w-2" : "inset-x-0 -top-1 h-2",
        )}
      />
    </div>
  );
}
