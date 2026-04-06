import type { TraceSpan } from "../types";

interface Props {
  root: TraceSpan;
  totalDuration: number;
  onSpanClick?: (span: TraceSpan) => void;
}

const SERVICE_COLORS: Record<string, string> = {
  default: "#3b82f6",
  api: "#8b5cf6",
  web: "#06b6d4",
  db: "#f59e0b",
  cache: "#10b981",
  queue: "#ef4444",
};

function getColor(service: string): string {
  for (const [key, color] of Object.entries(SERVICE_COLORS)) {
    if (service.toLowerCase().includes(key)) return color;
  }
  return SERVICE_COLORS.default;
}

function formatDuration(ns: number): string {
  if (ns < 1000) return `${ns}ns`;
  if (ns < 1_000_000) return `${(ns / 1000).toFixed(1)}us`;
  if (ns < 1_000_000_000) return `${(ns / 1_000_000).toFixed(1)}ms`;
  return `${(ns / 1_000_000_000).toFixed(2)}s`;
}

function SpanBar({
  span,
  depth,
  traceStart,
  totalDuration,
  onSpanClick,
}: {
  span: TraceSpan;
  depth: number;
  traceStart: number;
  totalDuration: number;
  onSpanClick?: (span: TraceSpan) => void;
}) {
  const startOffset = new Date(span.start_time).getTime() - traceStart;
  const left = totalDuration > 0 ? (startOffset / totalDuration) * 100 : 0;
  const width =
    totalDuration > 0
      ? (span.duration / 1_000_000 / totalDuration) * 100
      : 100;

  return (
    <>
      <div
        className="relative h-7 mb-0.5"
        style={{ paddingLeft: `${depth * 20}px` }}
      >
        <div
          className="flame-bar absolute h-full rounded cursor-pointer flex items-center px-2 text-white text-xs truncate"
          style={{
            left: `${Math.max(left, 0)}%`,
            width: `${Math.max(width, 0.5)}%`,
            backgroundColor: getColor(span.service),
          }}
          title={`${span.service}: ${span.operation} (${formatDuration(span.duration)})`}
          onClick={() => onSpanClick?.(span)}
        >
          <span className="truncate">
            {span.service}: {span.operation} ({formatDuration(span.duration)})
          </span>
        </div>
      </div>
      {span.children?.map((child) => (
        <SpanBar
          key={child.span_id}
          span={child}
          depth={depth + 1}
          traceStart={traceStart}
          totalDuration={totalDuration}
          onSpanClick={onSpanClick}
        />
      ))}
    </>
  );
}

export default function FlameGraph({
  root,
  totalDuration,
  onSpanClick,
}: Props) {
  const traceStart = new Date(root.start_time).getTime();
  const totalMs = totalDuration / 1_000_000; // ns to ms

  return (
    <div className="bg-white rounded-lg border border-gray-200 p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-semibold text-gray-700">Trace Timeline</h3>
        <span className="text-xs text-gray-500">
          Total: {formatDuration(totalDuration)}
        </span>
      </div>
      <div className="relative overflow-x-auto">
        <SpanBar
          span={root}
          depth={0}
          traceStart={traceStart}
          totalDuration={totalMs}
          onSpanClick={onSpanClick}
        />
      </div>
    </div>
  );
}
