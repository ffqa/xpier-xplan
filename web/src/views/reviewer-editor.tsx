import { useState } from "react";
import { Loader2Icon, TriangleAlertIcon } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
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
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";
import { t } from "@/lib/i18n";
import { invalidate } from "@/lib/query";
import type { ReviewerInfo, ReviewerTemplate } from "@/lib/types";
import { useMeta } from "@/lib/ui-state";

/** Same rule as the server: lowercase, digits, dot, dash, underscore. */
const NAME_PATTERN = /^[a-z0-9][a-z0-9._-]{0,39}$/;

/** Commands that list the model names an agent accepts, keyed by program. */
const MODEL_LIST_COMMANDS: Record<string, string> = {
  "cursor-agent": "cursor-agent --list-models",
  devin: "devin models list",
  omp: "omp models",
  "kiro-cli": "kiro-cli chat --list-models",
};

/** Model names an agent accepts as aliases, keyed by program. */
const MODEL_EXAMPLES: Record<string, string> = {
  claude: "sonnet, opus",
  codebuddy: "sonnet, opus",
};

function modelHint(program: string, supportsModel: boolean): string {
  if (!supportsModel) {
    return t("Add a line with {model} to the command to choose a model");
  }
  if (MODEL_LIST_COMMANDS[program]) {
    return t("List them with: {command}", {
      command: MODEL_LIST_COMMANDS[program],
    });
  }
  if (MODEL_EXAMPLES[program]) {
    return t("e.g. {hint}", { hint: MODEL_EXAMPLES[program] });
  }
  return t("Empty = the agent's own default");
}

export type EditorTarget =
  { mode: "add" } | { mode: "edit"; reviewer: ReviewerInfo };

/** Add a third-party/custom reviewer or change model, agent and timeout. */
export function ReviewerEditor({
  target,
  globalTimeout,
  onClose,
}: {
  target: EditorTarget | null;
  globalTimeout: string;
  onClose: () => void;
}) {
  return (
    <Dialog open={target !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-xl">
        {target && (
          <ReviewerForm
            key={target.mode === "edit" ? target.reviewer.name : "add"}
            target={target}
            globalTimeout={globalTimeout}
            onClose={onClose}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function ReviewerForm({
  target,
  globalTimeout,
  onClose,
}: {
  target: EditorTarget;
  globalTimeout: string;
  onClose: () => void;
}) {
  const meta = useMeta();
  const existing = target.mode === "edit" ? target.reviewer : null;
  const [name, setName] = useState(existing?.name ?? "");
  const [description, setDescription] = useState(existing?.description ?? "");
  const [commandText, setCommandText] = useState(
    existing?.command.join("\n") ?? "",
  );
  const [model, setModel] = useState(existing?.model ?? "");
  const [agent, setAgent] = useState(existing?.agent ?? "");
  const [timeout, setTimeoutValue] = useState(existing?.timeout ?? "");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState<"save" | "test" | null>(null);

  const command = commandText
    .split("\n")
    .map((part) => part.trim())
    .filter(Boolean);
  const program = command[0] ?? "";
  const supportsModel = commandText.includes("{model}");
  const supportsAgent = commandText.includes("{agent}");
  const nameError =
    !existing && name && (!NAME_PATTERN.test(name) || name === "auto")
      ? t(
          "Use 1-40 lowercase letters, digits, dots, dashes or underscores (not auto).",
        )
      : "";
  const taken =
    !existing && meta.data?.reviewers.some((item) => item.name === name);

  const applyTemplate = (template: ReviewerTemplate | null) => {
    setName(template?.name ?? "");
    setDescription(template?.description ?? "");
    setCommandText(template ? template.command.join("\n") : "");
    setNote(template?.note ?? "");
  };

  const save = async (andTest: boolean) => {
    setBusy(andTest ? "test" : "save");
    try {
      await api.saveReviewer(name, {
        // An unchanged built-in description is sent empty so the server
        // keeps the preset's (translated) text.
        description:
          existing?.builtin && description === existing.description
            ? ""
            : description,
        command,
        model: supportsModel ? model : "",
        agent: supportsAgent ? agent : "",
        timeout,
      });
      invalidate("meta");
      if (andTest) {
        const result = await api.testReviewer(name);
        if (result.health.status === "ok") {
          toast.success(t("{name} is working", { name }));
        } else {
          toast.warning(`${name}: ${result.health_label}`, {
            description: result.health.detail,
          });
        }
        invalidate("meta");
      } else {
        toast.success(t("Saved {name}", { name }));
      }
      onClose();
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setBusy(null);
    }
  };

  const canSave =
    busy === null &&
    name !== "" &&
    !nameError &&
    !taken &&
    command.length > 0 &&
    (commandText.includes("{prompt}") || commandText.includes("{prompt_file}"));

  return (
    <div className="grid gap-3">
      <DialogHeader>
        <DialogTitle>
          {existing
            ? t("Edit {name}", { name: existing.name })
            : t("Add reviewer")}
        </DialogTitle>
        <DialogDescription>
          {t(
            "Any agent CLI that can answer a prompt without asking questions works. Keep it read-only: xplan only asks it to review.",
          )}
        </DialogDescription>
      </DialogHeader>

      {!existing && (
        <Field label={t("Start from")}>
          <div className="flex flex-wrap gap-1.5">
            {(meta.data?.reviewer_templates ?? []).map((template) => (
              <Button
                key={template.name}
                variant={name === template.name ? "secondary" : "outline"}
                size="sm"
                title={template.description}
                onClick={() => applyTemplate(template)}
              >
                {template.name}
              </Button>
            ))}
            <Button
              variant="ghost"
              size="sm"
              onClick={() => applyTemplate(null)}
            >
              {t("Blank")}
            </Button>
          </div>
          {note && <p className="text-[11px] text-muted-foreground">{note}</p>}
        </Field>
      )}

      {!existing && (
        <Field
          label={t("Name")}
          error={nameError || (taken ? t("This name is already used.") : "")}
        >
          <Input
            value={name}
            onChange={(event) => setName(event.target.value.trim())}
            placeholder="my-reviewer"
          />
        </Field>
      )}

      <Field label={t("Description")}>
        <Input
          value={description}
          onChange={(event) => setDescription(event.target.value)}
        />
      </Field>

      <Field
        label={t("Command (one argument per line)")}
        hint={t(
          "No shell: each line is passed as is. Placeholders: {prompt} or {prompt_file} (required), {path}, {xplan_home}, {model}, {agent}. A line whose {model} or {agent} is empty is left out.",
        )}
      >
        <Textarea
          className="font-mono text-[12px]"
          rows={Math.min(Math.max(command.length + 1, 4), 12)}
          value={commandText}
          onChange={(event) => setCommandText(event.target.value)}
          placeholder={"my-agent\n--print\n--model={model}\n{prompt}"}
          spellCheck={false}
        />
      </Field>

      <div className="grid grid-cols-2 gap-3">
        <Field label={t("Model")} hint={modelHint(program, supportsModel)}>
          <Input
            value={model}
            disabled={!supportsModel}
            onChange={(event) => setModel(event.target.value.trim())}
            placeholder={t("agent default")}
          />
        </Field>
        <Field
          label={t("Agent")}
          hint={
            supportsAgent
              ? existing?.builtin
                ? t("Empty = the preset's agent")
                : t("Empty = the agent's own default")
              : t("This command has no {agent} slot")
          }
        >
          <Input
            value={agent}
            disabled={!supportsAgent}
            onChange={(event) => setAgent(event.target.value.trim())}
            placeholder={t("agent default")}
          />
        </Field>
      </div>
      {supportsAgent && agent !== "" && agent !== existing?.agent && (
        <p className="flex items-start gap-1.5 text-[11px] text-warn">
          <TriangleAlertIcon className="mt-px size-3 shrink-0" />
          {t(
            "The agent decides which tools the reviewer gets. Pick one that cannot edit files.",
          )}
        </p>
      )}

      <Field
        label={t("Timeout")}
        hint={t(
          "Slow models or big projects may need longer, e.g. 40m or 1h. Empty = the global timeout.",
        )}
      >
        <Input
          className="w-40"
          value={timeout}
          onChange={(event) => setTimeoutValue(event.target.value.trim())}
          placeholder={t("global ({value})", { value: globalTimeout })}
        />
      </Field>

      <DialogFooter className="items-center">
        {existing?.modified && (
          <Badge variant="info" className="mr-auto">
            {t("changed from the preset")}
          </Badge>
        )}
        <Button variant="ghost" onClick={onClose} disabled={busy !== null}>
          {t("Cancel")}
        </Button>
        <Button
          variant="outline"
          disabled={!canSave}
          onClick={() => void save(false)}
        >
          {busy === "save" && <Loader2Icon className="animate-spin" />}
          {t("Save")}
        </Button>
        <Button disabled={!canSave} onClick={() => void save(true)}>
          {busy === "test" && <Loader2Icon className="animate-spin" />}
          {t("Save and test")}
        </Button>
      </DialogFooter>
    </div>
  );
}

function Field({
  label,
  hint,
  error,
  children,
}: {
  label: string;
  hint?: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="grid gap-1.5">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      {children}
      {error ? (
        <span className="text-[11px] text-crit">{error}</span>
      ) : (
        hint && (
          <span className="text-[11px] text-muted-foreground">{hint}</span>
        )
      )}
    </div>
  );
}
