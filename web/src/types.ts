// Shared types matching the Go backend models.

export interface MetricSample {
  timestamp: string;
  value: number;
  tags: Record<string, string>;
}

export interface MetricResult {
  name: string;
  samples: MetricSample[];
}

export interface LogRecord {
  timestamp: string;
  service: string;
  host: string;
  level: string;
  message: string;
  fields: Record<string, string>;
  source: string;
}

export interface LogSearchResult {
  records: LogRecord[];
  total: number;
}

export interface LogAggregation {
  key: string;
  count: number;
}

export interface TraceSpan {
  trace_id: string;
  span_id: string;
  parent_id: string;
  service: string;
  operation: string;
  start_time: string;
  duration: number; // nanoseconds
  status: string;
  tags: Record<string, string>;
  events: any[];
  children?: TraceSpan[];
}

export interface TraceResult {
  trace_id: string;
  root: TraceSpan | null;
  spans: TraceSpan[];
  duration: number;
  services: string[];
}

export interface TemplateVariable {
  name: string;
  label: string;
  values: string[];
  selected: string;
}
