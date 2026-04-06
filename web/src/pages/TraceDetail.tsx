import { useState, useEffect } from "react";
import { getTrace } from "../api";
import FlameGraph from "../components/FlameGraph";
import type { TraceResult, TraceSpan } from "../types";

function formatDuration(ns: number): string {
  if (ns < 1_000_000) return `${(ns / 1000).toFixed(0)}us`;
  if (ns < 1_000_000_000) return `${(ns / 1_000_000).toFixed(1)}ms`;
  return `${(ns / 1_000_000_000).toFixed(2)}s`;
}

export default function TraceDetail({ traceId }: { traceId: string }) {
  const [trace, setTrace] = useState<TraceResult | null>(null);
  const [selectedSpan, setSelectedSpan] = useState<TraceSpan | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!traceId) return;
    let cancelled = false;
    setLoading(true);
    fetch(`/api/v1/traces/${traceId}`)
      .then((res) => res.json())
      .then((t) => { if (!cancelled) setTrace(t); })
      .catch(() => { if (!cancelled) setTrace(null); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [traceId]);

  if (loading && !trace) {
    return <div className="text-center text-gray-400 py-12">Loading trace {traceId}...</div>;
  }

  if (!trace || !trace.root) {
    return (
      <div className="text-center text-gray-400 py-12">Trace not found</div>
    );
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-lg font-semibold text-gray-800">
            {trace.root.operation}
          </h2>
          <p className="text-sm text-gray-500 font-mono">{trace.trace_id}</p>
        </div>
        <div className="flex items-center gap-4 text-sm text-gray-600">
          <span>{trace.spans.length} spans</span>
          <span>{trace.services.join(", ")}</span>
          <span className="font-medium">
            {formatDuration(trace.duration)}
          </span>
        </div>
      </div>

      {/* Flame graph */}
      <FlameGraph
        root={trace.root}
        totalDuration={trace.duration}
        onSpanClick={setSelectedSpan}
      />

      {/* Span detail panel */}
      {selectedSpan && (
        <div className="mt-4 bg-white rounded-lg border border-gray-200 p-4">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-semibold text-gray-700">
              Span Detail
            </h3>
            <button
              onClick={() => setSelectedSpan(null)}
              className="text-gray-400 hover:text-gray-600 text-lg"
            >
              x
            </button>
          </div>
          <dl className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
            <div>
              <dt className="text-gray-500">Service</dt>
              <dd className="font-medium">{selectedSpan.service}</dd>
            </div>
            <div>
              <dt className="text-gray-500">Operation</dt>
              <dd className="font-medium">{selectedSpan.operation}</dd>
            </div>
            <div>
              <dt className="text-gray-500">Span ID</dt>
              <dd className="font-mono text-xs">{selectedSpan.span_id}</dd>
            </div>
            <div>
              <dt className="text-gray-500">Duration</dt>
              <dd className="font-medium">
                {formatDuration(selectedSpan.duration)}
              </dd>
            </div>
            <div>
              <dt className="text-gray-500">Status</dt>
              <dd>
                <span
                  className={`px-1.5 py-0.5 rounded text-xs font-medium ${
                    selectedSpan.status === "error"
                      ? "bg-red-100 text-red-700"
                      : selectedSpan.status === "ok"
                        ? "bg-green-100 text-green-700"
                        : "bg-gray-100 text-gray-600"
                  }`}
                >
                  {selectedSpan.status}
                </span>
              </dd>
            </div>
            <div>
              <dt className="text-gray-500">Start Time</dt>
              <dd className="font-mono text-xs">
                {new Date(selectedSpan.start_time).toISOString()}
              </dd>
            </div>
          </dl>

          {/* Tags */}
          {Object.keys(selectedSpan.tags || {}).length > 0 && (
            <div className="mt-3">
              <h4 className="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1">
                Tags
              </h4>
              <div className="flex flex-wrap gap-1">
                {Object.entries(selectedSpan.tags).map(([k, v]) => (
                  <span
                    key={k}
                    className="px-2 py-0.5 bg-gray-100 rounded text-xs"
                  >
                    {k}={v}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Span list */}
      <div className="mt-4 bg-white rounded-lg border border-gray-200 overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 border-b border-gray-200">
            <tr>
              <th className="text-left px-3 py-2 font-medium text-gray-600">
                Service
              </th>
              <th className="text-left px-3 py-2 font-medium text-gray-600">
                Operation
              </th>
              <th className="text-left px-3 py-2 font-medium text-gray-600">
                Duration
              </th>
              <th className="text-left px-3 py-2 font-medium text-gray-600">
                Status
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {trace.spans.map((span) => (
              <tr
                key={span.span_id}
                className={`hover:bg-gray-50 cursor-pointer ${selectedSpan?.span_id === span.span_id ? "bg-lens-50" : ""}`}
                onClick={() => setSelectedSpan(span)}
              >
                <td className="px-3 py-1.5 font-medium">{span.service}</td>
                <td className="px-3 py-1.5">{span.operation}</td>
                <td className="px-3 py-1.5 font-mono text-xs">
                  {formatDuration(span.duration)}
                </td>
                <td className="px-3 py-1.5">
                  <span
                    className={`px-1.5 py-0.5 rounded text-xs font-medium ${
                      span.status === "error"
                        ? "bg-red-100 text-red-700"
                        : span.status === "ok"
                          ? "bg-green-100 text-green-700"
                          : "bg-gray-100 text-gray-600"
                    }`}
                  >
                    {span.status}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
