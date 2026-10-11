import { routeHref } from "@/lib/route";

/** Resolves a Markdown href; null means "render as plain text". */
export type HrefResolver = (href: string) => string | null;

const externalPattern = /^(https?:|mailto:|#)/i;
const lineSuffix = /(:\d+(-\d+)?|#L\d+(-L?\d+)?)$/;

/**
 * Agents cite files as absolute paths with line numbers
 * ("/abs/project/bin/x.py:584"). Inside the project they open in the Docs tab;
 * anything outside the project root is not linkable from the browser.
 */
export function projectFileResolver(
  slug: string,
  projectPath: string,
): HrefResolver {
  const root = projectPath ? projectPath.replace(/\/+$/, "") + "/" : null;
  const docHref = (relativePath: string) =>
    routeHref({ view: "project", slug, tab: "docs", arg: relativePath });

  return (href) => {
    if (externalPattern.test(href)) return href;
    const path = safeDecode(href).replace(lineSuffix, "");
    if (path.startsWith("/")) {
      if (!root || !path.startsWith(root)) return null;
      return docHref(path.slice(root.length));
    }
    return docHref(path.replace(/^\.\//, ""));
  };
}

function safeDecode(href: string): string {
  try {
    return decodeURI(href);
  } catch {
    return href;
  }
}
