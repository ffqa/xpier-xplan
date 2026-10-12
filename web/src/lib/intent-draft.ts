import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import { api } from "@/lib/api";
import { jobActive } from "@/lib/format";
import { invalidate, useQuery } from "@/lib/query";
import { refreshProject } from "@/lib/ui-state";

/**
 * The project's latest intent-draft job, polled while it runs. When a run
 * finishes the project is refreshed, because an empty intent.md has just been
 * filled in.
 */
export function useIntentDraft(slug: string) {
  const [polling, setPolling] = useState(false);
  const [starting, setStarting] = useState(false);
  const reviews = useQuery(`reviews:${slug}`, () => api.reviews(slug), {
    refetchInterval: polling ? 2500 : 20000,
  });
  const job = reviews.data?.jobs.find((item) => item.kind === "intent");
  const running = job !== undefined && jobActive(job.status);

  const watched = useRef<string | null>(null);
  useEffect(() => {
    setPolling(running);
    if (!job) return;
    if (running) {
      watched.current = job.id;
      return;
    }
    if (watched.current !== job.id) return;
    watched.current = null;
    refreshProject(slug);
    if (job.status === "done")
      toast.success(job.summary || "Intent draft ready");
    else if (job.status === "failed")
      toast.error(job.error || "Intent draft failed");
  }, [job, running, slug]);

  const start = async () => {
    setStarting(true);
    try {
      const created = await api.startReview(slug, "intent");
      toast.info(`Drafting the intent with ${created.reviewer} (read-only)…`);
      invalidate(`reviews:${slug}`, "overview");
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setStarting(false);
    }
  };

  const cancel = async () => {
    if (!job) return;
    try {
      await api.cancelReview(slug, job.id);
      invalidate(`reviews:${slug}`);
    } catch (error) {
      toast.error((error as Error).message);
    }
  };

  return { job, running, starting, start, cancel };
}
