// Simple in-memory cache for API query results.
// Persists across route changes so pages load instantly on revisit.

const cache = new Map<string, { data: any; timestamp: number }>();
const TTL = 30_000; // 30 seconds

export function getCached<T>(key: string): T | null {
  const entry = cache.get(key);
  if (!entry) return null;
  if (Date.now() - entry.timestamp > TTL) {
    cache.delete(key);
    return null;
  }
  return entry.data as T;
}

export function setCache(key: string, data: any): void {
  cache.set(key, { data, timestamp: Date.now() });
}
