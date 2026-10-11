import {
  BugIcon,
  CompassIcon,
  FlaskConicalIcon,
  Loader2Icon,
  MessageCircleQuestionIcon,
  RocketIcon,
  ShieldCheckIcon,
  type LucideIcon,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Tip } from "@/components/ui/tooltip";
import {
  levelDot,
  levelText,
  severityVariant,
  statusColor,
  verdictLabel,
  verdictVariant,
} from "@/lib/format";
import type {
  Flag,
  JobStatus,
  Level,
  Severity,
  Status,
  Verdict,
} from "@/lib/types";
import { cn } from "@/lib/utils";

export const kindIcon: Record<string, LucideIcon> = {
  drift: CompassIcon,
  code: BugIcon,
  security: ShieldCheckIcon,
  deploy: RocketIcon,
  test: FlaskConicalIcon,
  ask: MessageCircleQuestionIcon,
};

export function KindIcon({
  kind,
  className,
}: {
  kind: string;
  className?: string;
}) {
  const Icon = kindIcon[kind] ?? BugIcon;
  return <Icon className={cn("size-3.5", className)} />;
}

export function HealthDot({
  level,
  className,
}: {
  level: Level;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-block size-2 shrink-0 rounded-full",
        levelDot[level],
        className,
      )}
    />
  );
}

export function StatusDot({
  status,
  className,
}: {
  status: Status;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-block size-2 shrink-0 rounded-[3px]",
        statusColor[status],
        className,
      )}
    />
  );
}

export function VerdictBadge({
  verdict,
  kind,
}: {
  verdict?: Verdict;
  kind?: string;
}) {
  return (
    <Badge variant={verdictVariant(verdict)}>
      {verdictLabel(verdict, kind)}
    </Badge>
  );
}

export function SeverityBadge({ severity }: { severity: Severity }) {
  return (
    <Badge variant={severityVariant[severity]} className="w-16 uppercase">
      {severity}
    </Badge>
  );
}

export function JobStatusBadge({ status }: { status: JobStatus }) {
  if (status === "running" || status === "queued") {
    return (
      <Badge variant="info">
        <Loader2Icon className="animate-spin" />
        {status}
      </Badge>
    );
  }
  if (status === "failed") return <Badge variant="crit">failed</Badge>;
  if (status === "canceled") return <Badge variant="muted">canceled</Badge>;
  return null;
}

export function FlagList({ flags, limit }: { flags: Flag[]; limit?: number }) {
  const shown = limit ? flags.slice(0, limit) : flags;
  if (shown.length === 0)
    return <p className="text-xs text-muted-foreground">No issues detected.</p>;
  return (
    <ul className="space-y-1.5">
      {shown.map((flag) => (
        <li
          key={flag.code + flag.message}
          className="flex items-start gap-2 text-[12.5px] leading-snug"
        >
          <HealthDot level={flag.level} className="mt-1.5" />
          <span className={cn(flag.level === "crit" && levelText.crit)}>
            {flag.message}
          </span>
        </li>
      ))}
    </ul>
  );
}

/** Small row of per-kind verdict dots (drift, code, security ...). */
export function VerdictDots({
  verdicts,
}: {
  verdicts: Record<string, Verdict>;
}) {
  const kinds = ["drift", "code", "security", "deploy", "test"].filter(
    (kind) => verdicts[kind],
  );
  if (kinds.length === 0) return null;
  return (
    <span className="flex items-center gap-1">
      {kinds.map((kind) => {
        const verdict = verdicts[kind];
        const color =
          verdict === "ok"
            ? "text-ok"
            : verdict === "attention"
              ? "text-warn"
              : verdict === "critical"
                ? "text-crit"
                : "text-muted-foreground";
        return (
          <Tip key={kind} label={`${kind}: ${verdictLabel(verdict, kind)}`}>
            <span className={cn("inline-flex", color)}>
              <KindIcon kind={kind} className="size-3" />
            </span>
          </Tip>
        );
      })}
    </span>
  );
}
