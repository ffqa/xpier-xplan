import { useState } from "react";
import { BellIcon, Loader2Icon, SendIcon } from "lucide-react";
import { toast } from "sonner";

import { FieldLabel, Section } from "@/components/common";
import { Segmented } from "@/components/segmented";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { t } from "@/lib/i18n";
import type { NotifyPatch, NotifyResult, NotifySettings } from "@/lib/types";

/**
 * Notification settings. Each change is saved at once and kept out of the
 * page's draft, so the webhook URL never sits in browser state after saving
 * and unsaved edits elsewhere on the page are not reset.
 */
export function NotifySettingsCard({ initial }: { initial: NotifySettings }) {
  const [settings, setSettings] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [webhookDraft, setWebhookDraft] = useState<string | null>(null);

  const save = async (patch: NotifyPatch, done?: string) => {
    setBusy(true);
    try {
      const updated = await api.patchNotify(patch);
      setSettings(updated.notify);
      setWebhookDraft(null);
      if (done) toast.success(done);
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const sendTest = async () => {
    setBusy(true);
    try {
      const { results } = await api.testNotify();
      results.forEach(reportResult);
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const editingWebhook = webhookDraft !== null;
  return (
    <Section
      title={t("Notifications")}
      icon={<BellIcon />}
      actions={
        <Button
          variant="ghost"
          size="sm"
          disabled={busy || (!settings.desktop && !settings.webhook_set)}
          onClick={() => void sendTest()}
        >
          {busy ? <Loader2Icon className="animate-spin" /> : <SendIcon />}
          {t("Send a test")}
        </Button>
      }
    >
      <div className="grid grid-cols-[180px_minmax(0,1fr)] items-center gap-x-4 gap-y-3">
        <FieldLabel hint={t("Failed reviews always notify")}>
          {t("Notify when")}
        </FieldLabel>
        <Segmented
          value={settings.min_verdict}
          disabled={busy}
          onChange={(min_verdict) => void save({ min_verdict })}
          options={[
            { value: "attention", label: t("Needs attention or worse") },
            { value: "critical", label: t("Critical only") },
          ]}
        />

        <FieldLabel
          hint={
            settings.desktop_available
              ? t("A system notification on this computer")
              : t("Not available on this system")
          }
        >
          {t("Desktop")}
        </FieldLabel>
        <Segmented
          value={settings.desktop ? "on" : "off"}
          disabled={busy || !settings.desktop_available}
          onChange={(value) => void save({ desktop: value === "on" })}
          options={[
            { value: "on", label: t("On") },
            { value: "off", label: t("Off") },
          ]}
        />

        <FieldLabel
          hint={t(
            "Slack, Feishu/Lark, DingTalk, WeCom, Discord or any JSON endpoint",
          )}
        >
          {t("Chat webhook")}
        </FieldLabel>
        {settings.webhook_set && !editingWebhook ? (
          <div className="flex items-center gap-2">
            <code className="min-w-0 truncate font-mono text-xs text-muted-foreground">
              {settings.webhook_hint}
            </code>
            <Button
              variant="ghost"
              size="sm"
              disabled={busy}
              onClick={() => setWebhookDraft("")}
            >
              {t("Replace")}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              disabled={busy}
              onClick={() => {
                if (window.confirm(t("Remove the webhook?")))
                  void save({ webhook_url: "" }, t("Webhook removed"));
              }}
            >
              {t("Remove")}
            </Button>
          </div>
        ) : (
          <form
            className="flex items-center gap-2"
            onSubmit={(event) => {
              event.preventDefault();
              const webhook = (webhookDraft ?? "").trim();
              if (webhook)
                void save({ webhook_url: webhook }, t("Webhook saved"));
            }}
          >
            <Input
              className="flex-1 font-mono text-xs"
              type="url"
              placeholder="https://hooks.slack.com/services/…"
              value={webhookDraft ?? ""}
              onChange={(event) => setWebhookDraft(event.target.value)}
            />
            <Button
              type="submit"
              size="sm"
              variant="outline"
              disabled={busy || !(webhookDraft ?? "").trim()}
            >
              {t("Save")}
            </Button>
            {settings.webhook_set && (
              <Button
                type="button"
                size="sm"
                variant="ghost"
                onClick={() => setWebhookDraft(null)}
              >
                {t("Cancel")}
              </Button>
            )}
          </form>
        )}
      </div>
      <p className="mt-3 text-xs text-muted-foreground">
        {t(
          "Notifications come from the running xplan, so they need `xplan service install` (or an open `xplan serve`). The webhook URL is stored in config.yaml and never shown again. For DingTalk, add the keyword xplan to the bot.",
        )}
      </p>
    </Section>
  );
}

function reportResult(result: NotifyResult) {
  const channel =
    result.channel === "desktop" ? t("Desktop") : t("Chat webhook");
  if (result.ok) {
    toast.success(t("{channel}: sent", { channel }));
    return;
  }
  toast.error(t("{channel}: {error}", { channel, error: result.error ?? "" }));
}
