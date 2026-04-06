import { useState, useEffect, useRef } from "react";
import { searchLogs } from "../api";
import type { LogRecord } from "../types";
import { getCached, setCache } from "../stores/queryCache";

const LEVELS = ["", "debug", "info", "warn", "error", "fatal"];
const LEVEL_COLORS: Record<string, string> = {
  debug: "bg-gray-100 text-gray-600",
  info: "bg-blue-100 text-blue-700",
  warn: "bg-yellow-100 text-yellow-800",
  error: "bg-red-100 text-red-700",
  fatal: "bg-red-200 text-red-900",
};

export default function LogExplorer() {
  const cached = getCached<{ records: LogRecord[]; total: number }>("logs");
  const [records, setRecords] = useState<LogRecord[]>(cached?.records || []);
  const [total, setTotal] = useState(cached?.total || 0);
  const [service, setService] = useState("");
  const [level, setLevel] = useState("");
  const [search, setSearch] = useState("");
  const [tailMode, setTailMode] = useState(false);
  const [loading, setLoading] = useState(!cached);
  const bottomRef = useRef<HTMLDivElement>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);

  const fetchLogs = async () => {
    if (records.length === 0) setLoading(true);
    try {
      const result = await searchLogs({
        service,
        level,
        search,
        limit: 100,
        order: "desc",
      });
      const r = result.records || [];
      const t = result.total || 0;
      setRecords(r);
      setTotal(t);
      setCache("logs:" + service + level + search, { records: r, total: t });
    } catch {
      // silently retry on next interval
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchLogs();
  }, [service, level, search]);

  useEffect(() => {
    if (tailMode) {
      intervalRef.current = setInterval(fetchLogs, 2000);
      return () => clearInterval(intervalRef.current);
    } else {
      clearInterval(intervalRef.current);
    }
  }, [tailMode, service, level, search]);

  useEffect(() => {
    if (tailMode && bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: "smooth" });
    }
  }, [records, tailMode]);

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold text-gray-800">Log Explorer</h2>
        <div className="flex items-center gap-2">
          <span className="text-sm text-gray-500">{total} results</span>
          <button
            onClick={() => setTailMode(!tailMode)}
            className={`px-3 py-1 text-sm rounded font-medium transition-colors ${
              tailMode
                ? "bg-green-500 text-white"
                : "bg-gray-200 text-gray-700 hover:bg-gray-300"
            }`}
          >
            {tailMode ? "Tailing..." : "Tail"}
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex gap-3 mb-4">
        <input
          type="text"
          placeholder="Search messages..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="flex-1 px-3 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-lens-500"
        />
        <input
          type="text"
          placeholder="Service"
          value={service}
          onChange={(e) => setService(e.target.value)}
          className="w-36 px-3 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-lens-500"
        />
        <select
          value={level}
          onChange={(e) => setLevel(e.target.value)}
          className="w-28 px-3 py-1.5 text-sm border border-gray-300 rounded bg-white focus:outline-none focus:ring-2 focus:ring-lens-500"
        >
          {LEVELS.map((l) => (
            <option key={l} value={l}>
              {l || "All levels"}
            </option>
          ))}
        </select>
        <button
          onClick={fetchLogs}
          disabled={loading}
          className="px-4 py-1.5 text-sm bg-lens-600 text-white rounded hover:bg-lens-700 disabled:opacity-50"
        >
          Search
        </button>
      </div>

      {/* Log table */}
      <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 border-b border-gray-200">
            <tr>
              <th className="text-left px-3 py-2 font-medium text-gray-600 w-44">
                Timestamp
              </th>
              <th className="text-left px-3 py-2 font-medium text-gray-600 w-24">
                Level
              </th>
              <th className="text-left px-3 py-2 font-medium text-gray-600 w-28">
                Service
              </th>
              <th className="text-left px-3 py-2 font-medium text-gray-600">
                Message
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {records.map((log, i) => (
              <tr key={i} className="hover:bg-gray-50">
                <td className="px-3 py-1.5 text-gray-500 font-mono text-xs whitespace-nowrap">
                  {new Date(log.timestamp).toLocaleString()}
                </td>
                <td className="px-3 py-1.5">
                  <span
                    className={`px-1.5 py-0.5 rounded text-xs font-medium ${LEVEL_COLORS[log.level] || "bg-gray-100"}`}
                  >
                    {log.level || "-"}
                  </span>
                </td>
                <td className="px-3 py-1.5 text-gray-700 font-medium">
                  {log.service}
                </td>
                <td className="px-3 py-1.5 text-gray-800 font-mono text-xs truncate max-w-xl">
                  {log.message}
                </td>
              </tr>
            ))}
            {records.length === 0 && (
              <tr>
                <td
                  colSpan={4}
                  className="px-3 py-8 text-center text-gray-400"
                >
                  {loading ? "Loading..." : "No logs found"}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <div ref={bottomRef} />
    </div>
  );
}
