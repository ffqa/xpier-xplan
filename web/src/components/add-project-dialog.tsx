import { useState } from "react";
import { FolderSearchIcon, Loader2Icon } from "lucide-react";
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
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/misc";
import { api } from "@/lib/api";
import { t, tn } from "@/lib/i18n";
import { invalidate } from "@/lib/query";
import { navigate } from "@/lib/route";
import { useOverview, useUI } from "@/lib/ui-state";

export function AddProjectDialog() {
  const ui = useUI();
  return (
    <Dialog open={ui.addOpen} onOpenChange={ui.setAddOpen}>
      <DialogContent className="sm:max-w-xl">
        {ui.addOpen && <AddProjectForm close={() => ui.setAddOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function AddProjectForm({ close }: { close: () => void }) {
  const overview = useOverview();
  const firstPath = overview.data?.cards[0]?.path ?? "";
  const [path, setPath] = useState("");
  const [name, setName] = useState("");
  const [root, setRoot] = useState(
    firstPath ? firstPath.replace(/\/[^/]+\/?$/, "") : "",
  );
  const [candidates, setCandidates] = useState<string[] | null>(null);
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);

  const addOne = async () => {
    if (!path.trim()) return;
    setBusy(true);
    try {
      const created = await api.addProject(path.trim(), name.trim());
      toast.success(
        t(
          "Registered {name}. Write its intent first — it is what xplan reviews against.",
          { name: created.name },
        ),
      );
      invalidate("overview");
      close();
      navigate({ view: "project", slug: created.slug, tab: "intent" });
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const discover = async () => {
    if (!root.trim()) return;
    setBusy(true);
    try {
      const result = await api.discover(root.trim());
      setCandidates(result.candidates);
      setPicked(new Set(result.candidates));
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const addPicked = async () => {
    setBusy(true);
    const failures: string[] = [];
    for (const candidate of picked) {
      try {
        await api.addProject(candidate);
      } catch (error) {
        failures.push(`${candidate}: ${(error as Error).message}`);
      }
    }
    setBusy(false);
    invalidate("overview");
    if (failures.length) toast.error(failures.join("\n"));
    else
      toast.success(
        tn(
          picked.size,
          "Registered {count} project",
          "Registered {count} projects",
        ),
      );
    close();
  };

  const togglePick = (candidate: string) => {
    const next = new Set(picked);
    if (next.has(candidate)) next.delete(candidate);
    else next.add(candidate);
    setPicked(next);
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t("Register a project")}</DialogTitle>
        <DialogDescription>
          {t(
            "xplan only reads the project folder. Its own notes live in ~/.xplan, never inside the project.",
          )}
        </DialogDescription>
      </DialogHeader>

      <form
        className="grid gap-2"
        onSubmit={(event) => {
          event.preventDefault();
          void addOne();
        }}
      >
        <label className="text-xs font-medium text-muted-foreground">
          {t("Project folder")}
        </label>
        <Input
          autoFocus
          value={path}
          onChange={(event) => setPath(event.target.value)}
          placeholder="/Users/you/code/my-project"
        />
        <div className="flex gap-2">
          <Input
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder={t("Display name (optional)")}
          />
          <Button type="submit" disabled={busy || !path.trim()}>
            {busy && <Loader2Icon className="animate-spin" />}
            {t("Register")}
          </Button>
        </div>
      </form>

      <Separator />

      <div className="grid gap-2">
        <label className="text-xs font-medium text-muted-foreground">
          {t("Or discover git repositories in a folder")}
        </label>
        <div className="flex gap-2">
          <Input
            value={root}
            onChange={(event) => setRoot(event.target.value)}
            placeholder="/Users/you/code"
          />
          <Button
            variant="outline"
            onClick={() => void discover()}
            disabled={busy || !root.trim()}
          >
            <FolderSearchIcon />
            {t("Discover")}
          </Button>
        </div>
        {candidates && candidates.length === 0 && (
          <p className="text-xs text-muted-foreground">
            {t("No unregistered git repositories found there.")}
          </p>
        )}
        {candidates && candidates.length > 0 && (
          <ul className="max-h-48 overflow-y-auto rounded-md border p-1">
            {candidates.map((candidate) => (
              <li key={candidate}>
                <label className="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-[12.5px] hover:bg-accent/60">
                  <input
                    type="checkbox"
                    checked={picked.has(candidate)}
                    onChange={() => togglePick(candidate)}
                    className="accent-primary"
                  />
                  <span className="truncate font-mono">{candidate}</span>
                </label>
              </li>
            ))}
          </ul>
        )}
      </div>

      {candidates && candidates.length > 0 && (
        <DialogFooter>
          <Button
            onClick={() => void addPicked()}
            disabled={busy || picked.size === 0}
          >
            {busy && <Loader2Icon className="animate-spin" />}
            {t("Register {count} selected", { count: picked.size })}
          </Button>
        </DialogFooter>
      )}
    </>
  );
}
