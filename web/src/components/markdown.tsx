import { memo } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import type { HrefResolver } from "@/lib/links";
import { cn } from "@/lib/utils";

/** Renders GitHub-flavoured Markdown with the app's prose styles (.md). */
export const Markdown = memo(function Markdown({
  children,
  className,
  resolveHref,
}: {
  children: string;
  className?: string;
  resolveHref?: HrefResolver;
}) {
  return (
    <div className={cn("md", className)}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ href = "", children: linkChildren }) => {
            const target = resolveHref ? resolveHref(href) : href;
            if (target === null) return <code>{linkChildren}</code>;
            const internal = target.startsWith("#");
            return (
              <a
                href={target}
                target={internal ? undefined : "_blank"}
                rel={internal ? undefined : "noreferrer"}
              >
                {linkChildren}
              </a>
            );
          },
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
});
