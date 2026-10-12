import { useEffect } from "react";

import { api } from "@/lib/api";
import { isLanguage, setLanguage, type Language } from "@/lib/i18n";
import { invalidate } from "@/lib/query";
import { useMeta } from "@/lib/ui-state";

/**
 * Switches the UI and the server together, so console replies, health flags
 * and reviewer status arrive in the same language, then refetches everything
 * the server already localized.
 */
export async function switchLanguage(next: Language) {
  await api.patchConfig({ language: next });
  setLanguage(next);
  invalidate("");
}

/** Follows the language saved in the server config (shared by every browser). */
export function useServerLanguage() {
  const serverLanguage = useMeta().data?.language;
  useEffect(() => {
    if (isLanguage(serverLanguage)) setLanguage(serverLanguage);
  }, [serverLanguage]);
}
