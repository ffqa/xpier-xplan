import type { VariantProps } from "class-variance-authority";

import type { badgeVariants } from "@/components/ui/badge";
import { t } from "@/lib/i18n";
import type { HealthStatus, Job, Meta, ReviewerInfo } from "@/lib/types";

/** Reviewer value that picks the first working preset at run time. */
export const AUTO_REVIEWER = "auto";

type BadgeVariant = NonNullable<VariantProps<typeof badgeVariants>["variant"]>;

export const healthVariant: Record<HealthStatus, BadgeVariant> = {
  ok: "ok",
  auth: "warn",
  missing: "muted",
  timeout: "warn",
  error: "crit",
};

/** A configured reviewer value for people: "auto → codex" or the name. */
export function reviewerLabel(name: string, meta?: Meta | null): string {
  if (name !== AUTO_REVIEWER) return name;
  const choice = meta?.auto_choice?.name;
  return choice
    ? t("auto → {name}", { name: choice })
    : t("auto (nothing installed)");
}

/** The reviewer of a job: "codex (auto)" once auto picked one. */
export function jobReviewer(job: Pick<Job, "reviewer" | "reviewer_requested">) {
  if (job.reviewer === AUTO_REVIEWER) return t("the first working reviewer");
  if (job.reviewer_requested === AUTO_REVIEWER) {
    return t("{name} (auto)", { name: job.reviewer });
  }
  return job.reviewer;
}

/** Reviewers in the order "auto" tries them, following a draft order first. */
export function orderReviewers(
  reviewers: ReviewerInfo[],
  order: string[],
): ReviewerInfo[] {
  const byName = new Map(
    reviewers.map((reviewer) => [reviewer.name, reviewer]),
  );
  const ordered = order.flatMap((name) => byName.get(name) ?? []);
  const listed = new Set(ordered.map((reviewer) => reviewer.name));
  return [...ordered, ...reviewers.filter((r) => !listed.has(r.name))];
}
