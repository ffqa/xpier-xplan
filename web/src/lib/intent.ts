/**
 * Body of the first "## <prefix>..." section of a Markdown document, stopping
 * at the next level-2 heading or horizontal rule. Empty when missing.
 */
export function sectionBody(markdown: string, headingPrefix: string): string {
  const lines = markdown.split("\n");
  const start = lines.findIndex((line) =>
    line.trim().toLowerCase().startsWith(`## ${headingPrefix.toLowerCase()}`),
  );
  if (start < 0) return "";
  const body: string[] = [];
  for (const line of lines.slice(start + 1)) {
    const trimmed = line.trim();
    if (trimmed.startsWith("## ") || trimmed === "---") break;
    if (trimmed.startsWith("<!--")) continue;
    body.push(line);
  }
  return body.join("\n").trim();
}

/** The sections shown in the re-entry brief, in display order. */
export const BRIEF_SECTIONS = [
  { prefix: "Why", label: "Why" },
  { prefix: "Goal", label: "Goal" },
  { prefix: "Success", label: "Success looks like" },
  { prefix: "Non-goals", label: "Non-goals" },
] as const;
