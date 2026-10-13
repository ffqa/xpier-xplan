import type {
  ADR,
  Brief,
  Card,
  Config,
  ConsoleResult,
  Doc,
  DraftOutcome,
  Entry,
  HistoryItem,
  InboxItem,
  Intent,
  Job,
  Meta,
  Project,
  ReviewerDef,
  ReviewerHealth,
  ReviewerInfo,
  TreeEntry,
} from "@/lib/types";

/** Error carrying the HTTP status and the server's message. */
export class ApiError extends Error {
  status: number;
  body: unknown;
  constructor(status: number, message: string, body: unknown) {
    super(message);
    this.status = status;
    this.body = body;
  }
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const response = await fetch(path, {
    method,
    // X-Xplan is required by the server for every state-changing request.
    headers: { "Content-Type": "application/json", "X-Xplan": "1" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (response.status === 204) return undefined as T;
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new ApiError(
      response.status,
      (data as { error?: string }).error ?? response.statusText,
      data,
    );
  }
  return data as T;
}

const enc = encodeURIComponent;
const p = (slug: string) => `/api/projects/${enc(slug)}`;

export const api = {
  meta: () => request<Meta>("GET", "/api/meta"),
  config: () => request<Config>("GET", "/api/config"),
  patchConfig: (patch: Partial<Config>) =>
    request<Config>("PATCH", "/api/config", patch),
  testReviewer: (name: string) =>
    request<{ name: string; health: ReviewerHealth; health_label: string }>(
      "POST",
      `/api/reviewers/${enc(name)}/test`,
    ),
  testReviewers: () =>
    request<{
      results: Record<string, ReviewerHealth>;
      reviewers: ReviewerInfo[];
    }>("POST", "/api/reviewers/test"),
  /** Adds a custom reviewer or changes an existing one. */
  saveReviewer: (name: string, def: ReviewerDef) =>
    request<{ reviewer: ReviewerInfo; reviewers: ReviewerInfo[] }>(
      "PUT",
      `/api/reviewers/${enc(name)}`,
      def,
    ),
  /** Deletes a custom reviewer, or resets a built-in one. */
  removeReviewer: (name: string) =>
    request<{ reviewers: ReviewerInfo[] }>(
      "DELETE",
      `/api/reviewers/${enc(name)}`,
    ),
  overview: () =>
    request<{ cards: Card[]; active_reviews: Job[] }>("GET", "/api/overview"),
  inbox: (min: string) =>
    request<{ items: InboxItem[] }>("GET", `/api/inbox?min=${enc(min)}`),
  activeReviews: () => request<{ jobs: Job[] }>("GET", "/api/reviews/active"),
  discover: (root: string) =>
    request<{ candidates: string[] }>("POST", "/api/discover", { root }),
  addProject: (path: string, name = "") =>
    request<Project>("POST", "/api/projects", { path, name }),

  project: (slug: string) => request<Project>("GET", p(slug)),
  patchProject: (slug: string, patch: Partial<Project>) =>
    request<Project>("PATCH", p(slug), patch),
  removeProject: (slug: string) => request<void>("DELETE", p(slug)),
  brief: (slug: string) => request<Brief>("GET", `${p(slug)}/brief`),
  scan: (slug: string) =>
    request<{ docs: number; duration_ms: number }>("POST", `${p(slug)}/scan`),
  visit: (slug: string) => request<unknown>("POST", `${p(slug)}/visit`),

  docs: (slug: string) =>
    request<{
      docs: Doc[];
      kinds: string[];
      truncated: boolean;
      scanned_at: string;
    }>("GET", `${p(slug)}/docs`),
  file: (slug: string, path: string) =>
    request<{ path: string; kind: string; content: string }>(
      "GET",
      `${p(slug)}/file?path=${enc(path)}`,
    ),
  tree: (slug: string, path: string) =>
    request<{ entries: TreeEntry[] }>(
      "GET",
      `${p(slug)}/tree?path=${enc(path)}`,
    ),

  intent: (slug: string) => request<Intent>("GET", `${p(slug)}/intent`),
  saveIntent: (slug: string, content: string) =>
    request<Intent>("PUT", `${p(slug)}/intent`, { content }),
  draftIntents: () =>
    request<{ queued: DraftOutcome[]; skipped: DraftOutcome[] }>(
      "POST",
      "/api/intent-drafts",
    ),
  adrs: (slug: string) => request<{ adrs: ADR[] }>("GET", `${p(slug)}/adrs`),
  createADR: (slug: string, title: string, context = "") =>
    request<ADR>("POST", `${p(slug)}/adrs`, { title, context }),
  saveADR: (slug: string, id: string, content: string) =>
    request<ADR>("PUT", `${p(slug)}/adrs/${enc(id)}`, { content }),
  journal: (slug: string) =>
    request<{ entries: Entry[] }>("GET", `${p(slug)}/journal`),
  addJournal: (slug: string, entry: Partial<Entry>) =>
    request<Entry>("POST", `${p(slug)}/journal`, entry),

  reviews: (slug: string) =>
    request<{ jobs: Job[] }>("GET", `${p(slug)}/reviews`),
  startReview: (slug: string, kind: string, question = "") =>
    request<Job>("POST", `${p(slug)}/reviews`, { kind, question }),
  review: (slug: string, id: string) =>
    request<{ job: Job; report: string }>(
      "GET",
      `${p(slug)}/reviews/${enc(id)}`,
    ),
  cancelReview: (slug: string, id: string) =>
    request<unknown>("POST", `${p(slug)}/reviews/${enc(id)}/cancel`),

  consoleHistory: (slug: string) =>
    request<{ items: HistoryItem[] }>("GET", `${p(slug)}/console`),
  runConsole: (slug: string, input: string) =>
    request<ConsoleResult>("POST", `${p(slug)}/console`, { input }),
};
