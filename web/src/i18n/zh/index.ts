import { project } from "./project";
import { settings } from "./settings";
import { shell } from "./shell";
import { views } from "./views";

/** Simplified Chinese catalog, keyed by the English source text. */
export const zh: Record<string, string> = {
  ...shell,
  ...views,
  ...project,
  ...settings,
};
