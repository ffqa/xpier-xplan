import {
  BotIcon,
  CircleAlertIcon,
  FolderIcon,
  LanguagesIcon,
  TriangleAlertIcon,
} from "lucide-react";
import { toast } from "sonner";

import { t, tn, useLanguage } from "@/lib/i18n";
import { switchLanguage } from "@/lib/language";
import { AUTO_REVIEWER, reviewerLabel } from "@/lib/reviewers";
import { navigate } from "@/lib/route";
import { useMeta, useOverview } from "@/lib/ui-state";

export function StatusBar() {
  const meta = useMeta();
  const overview = useOverview();
  const cards = overview.data?.cards ?? [];
  const critical = cards.filter((card) => card.health_level === "crit").length;
  const warning = cards.filter((card) => card.health_level === "warn").length;
  const defaultReviewer = meta.data?.default_reviewer;
  const autoUnverified =
    defaultReviewer === AUTO_REVIEWER && !meta.data?.auto_choice?.verified;

  return (
    <footer className="flex h-6 shrink-0 items-center gap-4 border-t bg-sidebar px-3 text-[11px] text-muted-foreground">
      <span
        className="flex items-center gap-1"
        title={t("xplan data directory")}
      >
        <FolderIcon className="size-3" />
        {meta.data?.home ?? "…"}
      </span>
      <span>{tn(cards.length, "{count} project", "{count} projects")}</span>
      {critical > 0 && (
        <span className="flex items-center gap-1 text-crit">
          <CircleAlertIcon className="size-3" />
          {t("{count} critical", { count: critical })}
        </span>
      )}
      {warning > 0 && (
        <span className="flex items-center gap-1 text-warn">
          <TriangleAlertIcon className="size-3" />
          {t("{count} need attention", { count: warning })}
        </span>
      )}
      {overview.error && (
        <span className="text-crit">
          {t("API unreachable: {message}", { message: overview.error.message })}
        </span>
      )}
      <button
        type="button"
        className="ml-auto flex items-center gap-1 hover:text-foreground"
        title={
          autoUnverified
            ? t(
                "Default reviewer (read-only). Not checked yet: it is tested on first use, or test it in Settings.",
              )
            : t("Default reviewer (read-only)")
        }
        onClick={() => navigate({ view: "settings" })}
      >
        <BotIcon className="size-3" />
        {defaultReviewer ? reviewerLabel(defaultReviewer, meta.data) : "…"}
        {autoUnverified && <span className="text-warn">?</span>}
      </button>
      <span>
        {t("reports in {language}", {
          language: meta.data?.report_language
            ? t(meta.data.report_language)
            : "…",
        })}
      </span>
      <LanguageToggle />
      <span>v{meta.data?.version ?? "…"}</span>
    </footer>
  );
}

/** One click flips English and 简体中文 for the UI and server messages. */
function LanguageToggle() {
  const language = useLanguage();
  const next = language === "zh" ? "en" : "zh";
  const flip = () => {
    switchLanguage(next).catch((error: Error) => toast.error(error.message));
  };
  return (
    <button
      type="button"
      className="flex items-center gap-1 hover:text-foreground"
      title={next === "zh" ? "切换到简体中文" : "Switch to English"}
      onClick={flip}
    >
      <LanguagesIcon className="size-3" />
      {language === "zh" ? "中文" : "EN"}
    </button>
  );
}
