import { useState } from "react";
import { BrainIcon, CopyIcon } from "lucide-react";
import { toast } from "sonner";

import { ErrorBlock, LoadingBlock, Section } from "@/components/common";
import { Segmented } from "@/components/segmented";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { t } from "@/lib/i18n";
import { useQuery } from "@/lib/query";
import type { McpInfo } from "@/lib/types";

type Client = "cursor" | "claude" | "codex" | "stdio";

const AGENT_RULE =
  "At the start of a session, call xplan_context with your working directory and follow the intent it returns. " +
  "Before you stop, call xplan_park with where you stopped and the next step. " +
  "Propose decisions with xplan_propose_adr; never edit the intent.";

/** Ready-to-paste setup for each agent, built from the live endpoint. */
function clientSetup(client: Client, info: McpInfo) {
  switch (client) {
    case "cursor":
      return {
        where: "~/.cursor/mcp.json",
        code: JSON.stringify(
          { mcpServers: { xplan: { url: info.url } } },
          null,
          2,
        ),
      };
    case "claude":
      return {
        where: t("Terminal"),
        code: `claude mcp add --transport http --scope user xplan ${info.url}`,
      };
    case "codex":
      return {
        where: "~/.codex/config.toml",
        code: `[mcp_servers.xplan]\ncommand = ${JSON.stringify(info.binary)}\nargs = ["mcp"]`,
      };
    case "stdio":
      return {
        where: t("Any agent that starts MCP servers as commands"),
        code: JSON.stringify(
          { xplan: { command: info.binary, args: ["mcp"] } },
          null,
          2,
        ),
      };
  }
}

function copy(text: string) {
  navigator.clipboard.writeText(text).then(
    () => toast.success(t("Copied")),
    () => toast.error(t("Clipboard unavailable")),
  );
}

/** Connection guide for coding agents that use xplan as project memory. */
export function McpSettingsCard() {
  const info = useQuery("mcp-info", api.mcpInfo);
  const [client, setClient] = useState<Client>("cursor");

  return (
    <Section title={t("Agent memory (MCP)")} icon={<BrainIcon />}>
      {info.error ? (
        <ErrorBlock error={info.error} />
      ) : !info.data ? (
        <LoadingBlock />
      ) : (
        <McpGuide info={info.data} client={client} onClient={setClient} />
      )}
    </Section>
  );
}

function McpGuide({
  info,
  client,
  onClient,
}: {
  info: McpInfo;
  client: Client;
  onClient: (client: Client) => void;
}) {
  const setup = clientSetup(client, info);
  return (
    <div className="space-y-4">
      <p className="text-xs text-muted-foreground">
        {t(
          "Coding agents read each project's intent, decisions, park notes and review findings, and can leave park notes, notes and proposed decisions. They cannot change the intent. It is project memory, not a general-purpose store.",
        )}
      </p>

      <div className="space-y-2">
        <Segmented
          value={client}
          onChange={onClient}
          options={[
            { value: "cursor", label: "Cursor" },
            { value: "claude", label: "Claude Code" },
            { value: "codex", label: "Codex" },
            { value: "stdio", label: t("Other (stdio)") },
          ]}
        />
        <Snippet label={setup.where} code={setup.code} />
      </div>

      <Snippet
        label={t("Add to your agent rules (AGENTS.md, Cursor rules…)")}
        code={AGENT_RULE}
      />

      <div>
        <div className="mb-1.5 text-[13px]">{t("Tools agents see")}</div>
        <ul className="space-y-1.5">
          {info.tools.map((tool) => (
            <li key={tool.name} className="flex items-start gap-2 text-xs">
              <code className="shrink-0 font-mono">{tool.name}</code>
              <Badge variant={tool.read_only ? "muted" : "info"}>
                {tool.read_only ? t("reads") : t("writes")}
              </Badge>
              <span className="text-muted-foreground">{tool.description}</span>
            </li>
          ))}
        </ul>
      </div>

      <p className="text-xs text-muted-foreground">
        {t(
          "Agents reach xplan at {url}, so keep it running with `xplan service install`. Only local clients are accepted.",
          { url: info.url },
        )}
      </p>
    </div>
  );
}

function Snippet({ label, code }: { label: string; code: string }) {
  return (
    <div className="rounded-md border">
      <div className="flex items-center justify-between border-b px-2.5 py-1">
        <span className="font-mono text-[11px] text-muted-foreground">
          {label}
        </span>
        <Button variant="ghost" size="sm" onClick={() => copy(code)}>
          <CopyIcon />
          {t("Copy")}
        </Button>
      </div>
      <pre className="overflow-x-auto px-2.5 py-2 font-mono text-xs whitespace-pre-wrap">
        {code}
      </pre>
    </div>
  );
}
