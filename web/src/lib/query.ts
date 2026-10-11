import { useCallback, useEffect, useRef, useSyncExternalStore } from "react";

/**
 * A tiny keyed data cache with polling and prefix invalidation, enough for a
 * local single-user dashboard. Keys look like "brief:xteam" so that
 * invalidate("brief:") refreshes every brief currently on screen.
 */

interface CacheEntry {
  data?: unknown;
  error?: Error;
  loading: boolean;
  updatedAt: number;
  inflight?: Promise<void>;
  fetcher?: () => Promise<unknown>;
  listeners: Set<() => void>;
  snapshot: QueryState<unknown>;
}

export interface QueryState<T> {
  data?: T;
  error?: Error;
  loading: boolean;
}

const cache = new Map<string, CacheEntry>();

function entryFor(key: string): CacheEntry {
  let entry = cache.get(key);
  if (!entry) {
    entry = {
      loading: false,
      updatedAt: 0,
      listeners: new Set(),
      snapshot: { loading: false },
    };
    cache.set(key, entry);
  }
  return entry;
}

function publish(entry: CacheEntry) {
  entry.snapshot = {
    data: entry.data,
    error: entry.error,
    loading: entry.loading,
  };
  entry.listeners.forEach((listener) => listener());
}

async function load(key: string): Promise<void> {
  const entry = entryFor(key);
  if (!entry.fetcher) return;
  if (entry.inflight) return entry.inflight;
  entry.loading = true;
  publish(entry);
  entry.inflight = entry
    .fetcher()
    .then((data) => {
      entry.data = data;
      entry.error = undefined;
      entry.updatedAt = Date.now();
    })
    .catch((error: Error) => {
      entry.error = error;
    })
    .finally(() => {
      entry.loading = false;
      entry.inflight = undefined;
      publish(entry);
    });
  return entry.inflight;
}

/** Refetch every mounted query whose key starts with one of the prefixes. */
export function invalidate(...prefixes: string[]) {
  cache.forEach((entry, key) => {
    if (
      entry.listeners.size > 0 &&
      prefixes.some((prefix) => key.startsWith(prefix))
    ) {
      void load(key);
    }
  });
}

/** Optimistically replace cached data for a key. */
export function setQueryData<T>(
  key: string,
  update: (current: T | undefined) => T,
) {
  const entry = entryFor(key);
  entry.data = update(entry.data as T | undefined);
  publish(entry);
}

const idle: QueryState<never> = { loading: false };

/**
 * Subscribe to a cached query. Pass key=null to disable. refetchInterval (ms)
 * polls while mounted.
 */
export function useQuery<T>(
  key: string | null,
  fetcher: () => Promise<T>,
  options: { refetchInterval?: number | false } = {},
) {
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;

  const subscribe = useCallback(
    (listener: () => void) => {
      if (!key) return () => {};
      const entry = entryFor(key);
      entry.listeners.add(listener);
      return () => entry.listeners.delete(listener);
    },
    [key],
  );
  const getSnapshot = useCallback(
    () => (key ? entryFor(key).snapshot : idle),
    [key],
  );
  const state = useSyncExternalStore(subscribe, getSnapshot) as QueryState<T>;

  useEffect(() => {
    if (!key) return;
    const entry = entryFor(key);
    entry.fetcher = () => fetcherRef.current();
    if (entry.updatedAt === 0 || Date.now() - entry.updatedAt > 2000)
      void load(key);
  }, [key]);

  const interval = options.refetchInterval;
  useEffect(() => {
    if (!key || !interval) return;
    const timer = window.setInterval(() => void load(key), interval);
    return () => window.clearInterval(timer);
  }, [key, interval]);

  const refetch = useCallback(
    () => (key ? load(key) : Promise.resolve()),
    [key],
  );
  return { ...state, refetch };
}
