// API client for the Lens query layer.

import type {
  MetricSample,
  MetricResult,
  LogSearchResult,
  LogAggregation,
  TraceResult,
} from "./types";

const BASE = "/api/v1";

async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`API error ${res.status}: ${body}`);
  }
  return res.json();
}

// Metrics

export async function queryInstant(
  query: string,
  time?: string
): Promise<MetricSample[]> {
  const params = new URLSearchParams({ query });
  if (time) params.set("time", time);
  return fetchJSON(`${BASE}/query?${params}`);
}

export async function queryRange(
  query: string,
  start: string,
  end: string,
  step: string
): Promise<MetricResult[]> {
  const params = new URLSearchParams({ query, start, end, step });
  return fetchJSON(`${BASE}/query_range?${params}`);
}

// Logs

export async function searchLogs(params: {
  service?: string;
  level?: string;
  search?: string;
  start?: string;
  end?: string;
  limit?: number;
  offset?: number;
  order?: string;
}): Promise<LogSearchResult> {
  const sp = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== "") sp.set(k, String(v));
  });
  return fetchJSON(`${BASE}/logs?${sp}`);
}

export async function aggregateLogs(params: {
  group_by: string;
  service?: string;
  level?: string;
  search?: string;
  start?: string;
  end?: string;
}): Promise<LogAggregation[]> {
  const sp = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== "") sp.set(k, String(v));
  });
  return fetchJSON(`${BASE}/logs/aggregate?${sp}`);
}

// Traces

export async function getTrace(traceId: string): Promise<TraceResult> {
  return fetchJSON(`${BASE}/traces/${traceId}`);
}

export async function searchTraces(params: {
  service?: string;
  operation?: string;
  min_duration?: string;
  max_duration?: string;
  start?: string;
  end?: string;
  limit?: number;
}): Promise<TraceResult[]> {
  const sp = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== "") sp.set(k, String(v));
  });
  return fetchJSON(`${BASE}/traces?${sp}`);
}

// SSE Streaming

export function streamMetrics(
  query: string,
  interval: string,
  onData: (samples: MetricSample[]) => void,
  onError?: (err: Event) => void
): EventSource {
  const params = new URLSearchParams({ query, interval });
  const es = new EventSource(`${BASE}/stream?${params}`);
  es.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      onData(data);
    } catch {
      // skip invalid frames
    }
  };
  if (onError) es.onerror = onError;
  return es;
}
