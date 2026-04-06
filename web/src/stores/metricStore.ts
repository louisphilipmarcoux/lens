// Global metric data store that persists across route changes.
// SSE connections are managed here so they stay alive when navigating.

import { streamMetrics } from "../api";
import type { MetricSample } from "../types";

interface DataPoint {
  time: string;
  value: number;
}

interface StreamEntry {
  es: EventSource;
  data: DataPoint[];
  listeners: Set<() => void>;
  connected: boolean;
}

const MAX_POINTS = 60;
const streams = new Map<string, StreamEntry>();

function getKey(query: string, interval: string): string {
  return `${query}|${interval}`;
}

function getOrCreateStream(query: string, interval: string): StreamEntry {
  const key = getKey(query, interval);
  const existing = streams.get(key);
  if (existing) return existing;

  const entry: StreamEntry = {
    es: null!,
    data: [],
    listeners: new Set(),
    connected: false,
  };

  entry.es = streamMetrics(
    query,
    interval,
    (samples: MetricSample[]) => {
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
    },
    () => {
      entry.connected = false;
      entry.listeners.forEach((cb) => cb());
    }
  );

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
