import * as React from "react";
import { Loader2Icon } from "lucide-react";

import { cn } from "@/lib/utils";

/** Left-column label of a settings row, with an optional one-line hint. */
export function FieldLabel({
  children,
  hint,
}: {
  children: React.ReactNode;
  hint?: string;
}) {
  return (
    <div>
      <div className="text-[13px]">{children}</div>
      {hint && <div className="text-[11px] text-muted-foreground">{hint}</div>}
    </div>
  );
}

/** Titled block used across tabs (IDE "panel section" look). */
export function Section({
  title,
  icon,
  actions,
  children,
  className,
  bodyClassName,
}: {
  title: React.ReactNode;
  icon?: React.ReactNode;
  actions?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <section className={cn("rounded-lg border bg-card/60", className)}>
      <header className="flex h-9 items-center gap-2 border-b px-3">
        {icon && (
          <span className="text-muted-foreground [&_svg]:size-3.5">{icon}</span>
        )}
        <h3 className="text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">
          {title}
        </h3>
        <div className="ml-auto flex items-center gap-1">{actions}</div>
      </header>
      <div className={cn("p-3", bodyClassName)}>{children}</div>
    </section>
  );
}

export function Empty({
  icon,
  title,
  children,
  className,
}: {
  icon?: React.ReactNode;
  title: string;
  children?: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-2 p-8 text-center",
        className,
      )}
    >
      {icon && (
        <div className="text-muted-foreground/60 [&_svg]:size-8">{icon}</div>
      )}
      <p className="text-sm font-medium">{title}</p>
      {children && (
        <div className="max-w-sm text-xs text-muted-foreground">{children}</div>
      )}
    </div>
  );
}

export function Spinner({ className }: { className?: string }) {
  return (
    <Loader2Icon
      className={cn("size-4 animate-spin text-muted-foreground", className)}
    />
  );
}

export function LoadingBlock() {
  return (
    <div className="flex h-full items-center justify-center p-8">
      <Spinner />
    </div>
  );
}

export function ErrorBlock({ error }: { error: Error }) {
  return (
    <div className="m-4 rounded-md border border-crit/30 bg-crit/10 p-3 text-xs text-crit">
      {error.message}
    </div>
  );
}

/** Label/value pair for compact stat grids. */
export function Stat({
  label,
  value,
  hint,
}: {
  label: string;
  value: React.ReactNode;
  hint?: string;
}) {
  return (
    <div className="min-w-0" title={hint}>
      <div className="text-[11px] text-muted-foreground">{label}</div>
      <div className="truncate text-[13px] font-medium">{value}</div>
    </div>
  );
}
