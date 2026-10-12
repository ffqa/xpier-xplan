import { useSyncExternalStore } from "react";

import { zh } from "@/i18n/zh";

/**
 * Minimal gettext-style i18n: the English text is the key, so untranslated
 * strings (and text the server already localized) fall back to themselves.
 * Call t() at render time, never at module load: a language switch re-mounts
 * the UI but does not re-evaluate module-level constants.
 */

export type Language = "en" | "zh";

export const LANGUAGES: { id: Language; label: string }[] = [
  { id: "en", label: "English" },
  { id: "zh", label: "简体中文" },
];

const STORAGE_KEY = "xplan.language";
const catalogs: Record<Language, Record<string, string>> = { en: {}, zh };
const listeners = new Set<() => void>();

let current: Language = initialLanguage();
applyDocumentLanguage(current);

function initialLanguage(): Language {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (isLanguage(stored)) return stored;
  return navigator.language.toLowerCase().startsWith("zh") ? "zh" : "en";
}

function applyDocumentLanguage(language: Language) {
  document.documentElement.lang = language === "zh" ? "zh-CN" : "en";
}

export function isLanguage(value: unknown): value is Language {
  return value === "en" || value === "zh";
}

export function getLanguage(): Language {
  return current;
}

/** Switches the UI language and notifies subscribers (the app shell re-mounts). */
export function setLanguage(next: Language) {
  if (next === current) return;
  current = next;
  localStorage.setItem(STORAGE_KEY, next);
  applyDocumentLanguage(next);
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useLanguage(): Language {
  return useSyncExternalStore(subscribe, getLanguage);
}

/** Translates text and fills {name} placeholders from vars. */
export function t(
  text: string,
  vars?: Record<string, string | number>,
): string {
  const template = catalogs[current][text] ?? text;
  if (!vars) return template;
  return template.replace(/\{(\w+)\}/g, (placeholder, name: string) =>
    name in vars ? String(vars[name]) : placeholder,
  );
}

/** Picks the singular or plural English key before translating. */
export function tn(
  count: number,
  singular: string,
  plural: string,
  vars: Record<string, string | number> = {},
): string {
  return t(count === 1 ? singular : plural, { count, ...vars });
}
