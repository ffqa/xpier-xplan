import {
  BookOpenIcon,
  ChevronDownIcon,
  CompassIcon,
  CopyIcon,
  ExternalLinkIcon,
  FileTextIcon,
  FolderXIcon,
  HistoryIcon,
  LightbulbIcon,
  Loader2Icon,
  PlayIcon,
  RefreshCwIcon,
  Settings2Icon,
  SquareParkingIcon,
  TargetIcon,
} from "lucide-react";
import { toast } from "sonner";

import { Empty, ErrorBlock, LoadingBlock } from "@/components/common";
import { HealthDot, KindIcon, StatusDot } from "@/components/status";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tip } from "@/components/ui/tooltip";
import { ApiError, api } from "@/lib/api";
import { statusHint, statusLabel } from "@/lib/format";
import { invalidate, useQuery } from "@/lib/query";
import { navigate, PROJECT_TABS, type ProjectTab } from "@/lib/route";
import type { Brief, Status } from "@/lib/types";
import { refreshProject, useMeta, useUI } from "@/lib/ui-state";
import { BriefTab } from "@/views/project/brief-tab";
import { DecisionsTab } from "@/views/project/decisions-tab";
import { DocsTab } from "@/views/project/docs-tab";
import { IntentTab } from "@/views/project/intent-tab";
import { JournalTab } from "@/views/project/journal-tab";
import { ReviewsTab } from "@/views/project/reviews-tab";
import { ProjectSettingsTab } from "@/views/project/settings-tab";

const tabMeta: Record<ProjectTab, { label: string; icon: typeof TargetIcon }> =
  {
    brief: { label: "Brief", icon: CompassIcon },
    intent: { label: "Intent", icon: TargetIcon },
    decisions: { label: "Decisions", icon: LightbulbIcon },
    docs: { label: "Docs", icon: BookOpenIcon },
    reviews: { label: "Reviews", icon: FileTextIcon },
    journal: { label: "Journal", icon: HistoryIcon },
    settings: { label: "Settings", icon: Settings2Icon },
  };

export function ProjectView({
  slug,
  tab,
  arg,
}: {
  slug: string;
  tab: ProjectTab;
  arg?: string;
}) {
  const brief = useQuery(`brief:${slug}`, () => api.brief(slug), {
    refetchInterval: 20000,
  });

  if (brief.error && !brief.data) {
    if (brief.error instanceof ApiError && brief.error.status === 404) {
      return (
        <Empty
          icon={<FolderXIcon />}
          title="Project not found"
          className="h-full"
        >
          It may have been unregistered.{" "}
          <a href="#/" className="text-primary hover:underline">
            Back to overview
          </a>
        </Empty>
      );
    }
    return <ErrorBlock error={brief.error} />;
  }
  if (!brief.data) return <LoadingBlock />;

  return (
    <div className="flex h-full flex-col">
      <ProjectHeader brief={brief.data} />
      <Tabs
        value={tab}
        onValueChange={(value) =>
          navigate({ view: "project", slug, tab: value as ProjectTab })
        }
        className="min-h-0 flex-1"
      >
        <TabsList>
          {PROJECT_TABS.map((id) => {
            const Icon = tabMeta[id].icon;
            return (
              <TabsTrigger key={id} value={id}>
                <Icon />
                {tabMeta[id].label}
                <TabBadge id={id} brief={brief.data!} />
              </TabsTrigger>
            );
          })}
        </TabsList>
        <div className="min-h-0 flex-1 overflow-hidden">
          <TabBody slug={slug} tab={tab} arg={arg} brief={brief.data} />
        </div>
      </Tabs>
    </div>
  );
}

function TabBody({
  slug,
  tab,
  arg,
  brief,
}: {
  slug: string;
  tab: ProjectTab;
  arg?: string;
  brief: Brief;
}) {
  switch (tab) {
    case "intent":
      return <IntentTab slug={slug} />;
    case "decisions":
      return <DecisionsTab slug={slug} arg={arg} brief={brief} />;
    case "docs":
      return <DocsTab slug={slug} arg={arg} />;
    case "reviews":
      return <ReviewsTab slug={slug} arg={arg} projectPath={brief.card.path} />;
    case "journal":
      return <JournalTab slug={slug} />;
    case "settings":
      return <ProjectSettingsTab slug={slug} brief={brief} />;
    default:
      return <BriefTab slug={slug} brief={brief} />;
  }
}

function TabBadge({ id, brief }: { id: ProjectTab; brief: Brief }) {
  if (id === "intent" && !brief.card.intent_filled)
    return (
      <span
        className="size-1.5 rounded-full bg-warn"
        title="Intent not written yet"
      />
    );
  if (id === "reviews" && brief.active_reviews.length > 0)
    return <Loader2Icon className="animate-spin text-info" />;
  if (id === "docs" && brief.card.doc_count > 0)
    return (
      <span className="text-[10.5px] text-muted-foreground tabular-nums">
        {brief.card.doc_count}
      </span>
    );
  if (id === "decisions" && brief.adrs.length > 0)
    return (
      <span className="text-[10.5px] text-muted-foreground tabular-nums">
        {brief.adrs.length}
      </span>
    );
  return null;
}

function ProjectHeader({ brief }: { brief: Brief }) {
  const ui = useUI();
  const meta = useMeta();
  const { card } = brief;
  const slug = card.slug;

  const setStatus = async (status: Status) => {
    try {
      await api.patchProject(slug, { status });
      toast.success(`Moved to ${statusLabel[status]}`);
      refreshProject(slug);
    } catch (error) {
      toast.error((error as Error).message);
    }
  };

  const rescan = async () => {
    try {
      const result = await api.scan(slug);
      toast.success(`Scanned ${result.docs} docs in ${result.duration_ms}ms`);
      refreshProject(slug);
    } catch (error) {
      toast.error((error as Error).message);
    }
  };

  const openInEditor = async () => {
    try {
      const result = await api.runConsole(slug, "/open");
      if (result.ok) toast.success(result.output);
      else toast.error(result.output);
      invalidate(`console:${slug}`);
    } catch (error) {
      toast.error((error as Error).message);
    }
  };

  const startReview = async (kind: string) => {
    try {
      const job = await api.startReview(slug, kind);
      toast.success(`${kind} review queued with ${job.reviewer}`);
      refreshProject(slug);
      invalidate("overview");
      navigate({ view: "project", slug, tab: "reviews", arg: job.id });
    } catch (error) {
      toast.error((error as Error).message);
    }
  };

  const copyPath = () => {
    navigator.clipboard.writeText(card.path).then(
      () => toast.success("Path copied"),
      () => toast.error("Clipboard unavailable"),
    );
  };

  return (
    <header className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-1 border-b px-4 py-2.5">
      <div className="flex min-w-0 items-center gap-2">
        <Tip label={card.health[0]?.message ?? "Healthy"} side="bottom">
          <span className="inline-flex">
            <HealthDot level={card.health_level} className="size-2.5" />
          </span>
        </Tip>
        <h1 className="truncate text-[15px] font-semibold">{card.name}</h1>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={<Button variant="outline" size="xs" className="gap-1.5" />}
          >
            <StatusDot status={card.status} />
            {statusLabel[card.status]}
            <ChevronDownIcon className="size-3 text-muted-foreground" />
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            {(meta.data?.statuses ?? []).map((status) => (
              <DropdownMenuItem
                key={status}
                onClick={() => void setStatus(status)}
              >
                <StatusDot status={status} />
                <span>{statusLabel[status]}</span>
                <span className="ml-auto pl-4 text-[11px] text-muted-foreground">
                  {statusHint[status]}
                </span>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <button
        type="button"
        onClick={copyPath}
        title="Copy path"
        className="flex min-w-0 items-center gap-1 font-mono text-[11.5px] text-muted-foreground hover:text-foreground"
      >
        <span className="truncate">{card.path}</span>
        <CopyIcon className="size-3 shrink-0" />
      </button>

      <div className="hidden items-center gap-1 xl:flex">
        {(card.stack ?? []).slice(0, 5).map((item) => (
          <Badge key={item} variant="outline">
            {item}
          </Badge>
        ))}
      </div>

      <div className="ml-auto flex items-center gap-1">
        <Tip label="Open in Cursor" side="bottom">
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => void openInEditor()}
          >
            <ExternalLinkIcon />
          </Button>
        </Tip>
        <Tip label="Rescan docs, git and progress" side="bottom">
          <Button variant="ghost" size="icon-sm" onClick={() => void rescan()}>
            <RefreshCwIcon />
          </Button>
        </Tip>
        <Button variant="outline" size="sm" onClick={() => ui.openPark(slug)}>
          <SquareParkingIcon />
          Park
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger render={<Button size="sm" />}>
            <PlayIcon />
            Review
            <ChevronDownIcon className="size-3" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-64">
            {(meta.data?.review_kinds ?? [])
              .filter((kind) => kind.id !== "ask")
              .map((kind) => (
                <DropdownMenuItem
                  key={kind.id}
                  onClick={() => void startReview(kind.id)}
                >
                  <KindIcon kind={kind.id} />
                  <div className="min-w-0">
                    <div>{kind.title}</div>
                    <div className="truncate text-[11px] text-muted-foreground">
                      {kind.description}
                    </div>
                  </div>
                </DropdownMenuItem>
              ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => ui.seedConsole("")}>
              <KindIcon kind="ask" />
              Ask a question in the console
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
