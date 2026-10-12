import { useState } from "react";
import { Loader2Icon } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Kbd } from "@/components/ui/misc";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";
import { timeAgo } from "@/lib/format";
import { t } from "@/lib/i18n";
import { refreshProject, useOverview, useUI } from "@/lib/ui-state";

/** "Park" = write down where you are before switching to another project. */
export function ParkDialog() {
  const ui = useUI();
  return (
    <Dialog
      open={ui.parkSlug !== null}
      onOpenChange={(open) => !open && ui.openPark(null)}
    >
      <DialogContent className="sm:max-w-lg">
        {ui.parkSlug && (
          <ParkForm slug={ui.parkSlug} close={() => ui.openPark(null)} />
        )}
      </DialogContent>
    </Dialog>
  );
}

export function ParkForm({
  slug,
  close,
  compact,
}: {
  slug: string;
  close?: () => void;
  compact?: boolean;
}) {
  const overview = useOverview();
  const card = overview.data?.cards.find((item) => item.slug === slug);
  const previous = card?.last_park;
  const [stopped, setStopped] = useState("");
  const [next, setNext] = useState("");
  const [open, setOpen] = useState("");
  const [busy, setBusy] = useState(false);

  const save = async () => {
    if (!stopped.trim()) {
      toast.error(
        t("Say where you stopped — that is the part future-you needs most."),
      );
      return;
    }
    setBusy(true);
    try {
      await api.addJournal(slug, {
        kind: "park",
        stopped: stopped.trim(),
        next: next.trim(),
        open: open.trim(),
      });
      toast.success(
        t("Parked {name}. You will see this note first when you come back.", {
          name: card?.name ?? slug,
        }),
      );
      refreshProject(slug);
      setStopped("");
      setNext("");
      setOpen("");
      close?.();
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const onKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      void save();
    }
  };

  return (
    <div className="grid gap-3" onKeyDown={onKeyDown}>
      {!compact && (
        <DialogHeader>
          <DialogTitle>
            {t("Park {name}", { name: card?.name ?? slug })}
          </DialogTitle>
          <DialogDescription>
            {t(
              "Thirty seconds now saves the warm-up later. This is the first thing you will see when you come back.",
            )}
          </DialogDescription>
        </DialogHeader>
      )}
      <Field label={t("Where did you stop?")} required>
        <Textarea
          autoFocus={!compact}
          value={stopped}
          onChange={(event) => setStopped(event.target.value)}
          placeholder={t(
            "e.g. OAuth callback works locally; refresh-token rotation half done in TokenService",
          )}
          rows={2}
        />
      </Field>
      <Field label={t("Next concrete step")}>
        <Textarea
          value={next}
          onChange={(event) => setNext(event.target.value)}
          placeholder={
            previous?.next
              ? t("Last time: {next}", { next: previous.next })
              : t(
                  "e.g. write the rotation test, then wire it into the middleware",
                )
          }
          rows={2}
        />
      </Field>
      <Field label={t("Open questions / things not to forget")}>
        <Textarea
          value={open}
          onChange={(event) => setOpen(event.target.value)}
          placeholder={t("e.g. ask whether tokens must survive a deploy")}
          rows={2}
        />
      </Field>
      <DialogFooter className="items-center">
        {previous && (
          <span className="mr-auto text-xs text-muted-foreground">
            {t("Last parked {time}", { time: timeAgo(previous.at) })}
          </span>
        )}
        <span className="text-xs text-muted-foreground">
          <Kbd>⌘</Kbd> <Kbd>Enter</Kbd>
        </span>
        <Button onClick={() => void save()} disabled={busy}>
          {busy && <Loader2Icon className="animate-spin" />}
          {t("Park")}
        </Button>
      </DialogFooter>
    </div>
  );
}

function Field({
  label,
  required,
  children,
}: {
  label: string;
  required?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div className="grid gap-1.5">
      <span className="text-xs font-medium text-muted-foreground">
        {label}
        {required && <span className="text-crit"> *</span>}
      </span>
      {children}
    </div>
  );
}
