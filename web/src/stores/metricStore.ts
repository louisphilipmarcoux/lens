// Global metric data store that persists across route changes.
// Uses polling instead of SSE to avoid exhausting browser connection limits.

import { queryInstant } from "../api";

interface DataPoint {
  time: string;
  value: number;
}

interface StreamEntry {
  data: DataPoint[];
  listeners: Set<() => void>;
  connected: boolean;
  timer: ReturnType<typeof setInterval> | null;
}

const MAX_POINTS = 60;
const streams = new Map<string, StreamEntry>();

function getKey(query: string, interval: string): string {
  return `${query}|${interval}`;
}

function parseInterval(interval: string): number {
  const match = interval.match(/^(\d+)([sm])$/);
  if (!match) return 10000;
  const val = parseInt(match[1]);
  return match[2] === "m" ? val * 60000 : val * 1000;
}

function getOrCreateStream(query: string, interval: string): StreamEntry {
  const key = getKey(query, interval);
  const existing = streams.get(key);
  if (existing) return existing;

  const entry: StreamEntry = {
    data: [],
    listeners: new Set(),
    connected: false,
    timer: null,
  };

  const poll = async () => {
    try {
      const samples = await queryInstant(query);
      entry.connected = true;
      if (samples && samples.length > 0) {
        const avg =
          samples.reduce((s, m) => s + m.value, 0) / samples.length;
        entry.data = [
          ...entry.data,
          {
            time: new Date().toLocaleTimeString(),
            value: Math.round(avg * 100) / 100,
          },
        ].slice(-MAX_POINTS);
        entry.listeners.forEach((cb) => cb());
      }
    } catch {
      entry.connected = false;
      entry.listeners.forEach((cb) => cb());
    }
  };

  // Poll immediately, then on interval.
  poll();
  entry.timer = setInterval(poll, parseInterval(interval));

  streams.set(key, entry);
  return entry;
}

export function subscribe(
  query: string,
  interval: string,
  callback: () => void
): () => void {
  const entry = getOrCreateStream(query, interval);
  entry.listeners.add(callback);
  return () => {
    entry.listeners.delete(callback);
  };
}

export function getData(
  query: string,
  interval: string
): { data: DataPoint[]; connected: boolean } {
  const key = getKey(query, interval);
  const entry = streams.get(key);
  if (!entry) return { data: [], connected: false };
  return { data: entry.data, connected: entry.connected };
}
