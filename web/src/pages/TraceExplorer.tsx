import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import { searchTraces } from "../api";
import type { TraceResult } from "../types";

function formatDuration(ns: number): string {
  if (ns < 1_000_000) return `${(ns / 1000).toFixed(0)}us`;
  if (ns < 1_000_000_000) return `${(ns / 1_000_000).toFixed(1)}ms`;
  return `${(ns / 1_000_000_000).toFixed(2)}s`;
}

export default function TraceExplorer() {
  const [traces, setTraces] = useState<TraceResult[]>([]);
  const [service, setService] = useState("");
  const [operation, setOperation] = useState("");
  const [minDuration, setMinDuration] = useState("");
  const [loading, setLoading] = useState(false);

  const fetchTraces = async () => {
    setLoading(true);
    try {
      const results = await searchTraces({
        service,
        operation,
        min_duration: minDuration,
        limit: 20,
      });
      setTraces(results || []);
    } catch {
      setTraces([]);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchTraces();
  }, []);

  return (
    <div>
      <h2 className="text-lg font-semibold text-gray-800 mb-4">
        Trace Explorer
      </h2>

      {/* Filters */}
      <div className="flex gap-3 mb-4">
        <input
          type="text"
          placeholder="Service"
          value={service}
          onChange={(e) => setService(e.target.value)}
          className="w-40 px-3 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-lens-500"
        />
        <input
          type="text"
          placeholder="Operation"
          value={operation}
          onChange={(e) => setOperation(e.target.value)}
          className="w-40 px-3 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-lens-500"
        />
        <input
          type="text"
          placeholder="Min duration (e.g. 100ms)"
          value={minDuration}
          onChange={(e) => setMinDuration(e.target.value)}
          className="w-48 px-3 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-lens-500"
        />
        <button
          onClick={fetchTraces}
          disabled={loading}
          className="px-4 py-1.5 text-sm bg-lens-600 text-white rounded hover:bg-lens-700 disabled:opacity-50"
        >
          Search
        </button>
      </div>

      {/* Trace list */}
      <div className="space-y-2">
        {traces.map((trace) => (
          <Link
            key={trace.trace_id}
            to={`/traces/${trace.trace_id}`}
            className="block bg-white rounded-lg border border-gray-200 p-4 hover:border-lens-500 transition-colors"
          >
            <div className="flex items-center justify-between">
              <div>
                <span className="font-mono text-sm text-gray-500">
                  {trace.trace_id.slice(0, 16)}...
                </span>
                <span className="ml-3 text-sm font-medium text-gray-800">
                  {trace.root?.operation || "unknown"}
                </span>
              </div>
              <div className="flex items-center gap-4">
                <span className="text-xs text-gray-500">
                  {trace.spans.length} spans
                </span>
                <span className="text-xs text-gray-500">
                  {trace.services.join(", ")}
                </span>
                <span className="text-sm font-medium text-gray-700">
                  {formatDuration(trace.duration)}
                </span>
              </div>
            </div>
          </Link>
        ))}
        {traces.length === 0 && !loading && (
          <div className="text-center text-gray-400 py-12">
            No traces found
          </div>
        )}
        {loading && (
          <div className="text-center text-gray-400 py-12">Loading...</div>
        )}
      </div>
    </div>
  );
}
