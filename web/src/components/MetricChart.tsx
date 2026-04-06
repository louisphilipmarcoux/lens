import { useEffect, useState, useCallback } from "react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import { subscribe, getData } from "../stores/metricStore";

interface Props {
  title: string;
  query: string;
  interval?: string;
  color?: string;
  unit?: string;
}

export default function MetricChart({
  title,
  query,
  interval = "10s",
  color = "#3b82f6",
  unit = "",
}: Props) {
  const [, forceUpdate] = useState(0);
  const onData = useCallback(() => forceUpdate((n) => n + 1), []);

  useEffect(() => subscribe(query, interval, onData), [query, interval, onData]);

  const { data, connected } = getData(query, interval);

  return (
    <div className="bg-white rounded-lg border border-gray-200 p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-semibold text-gray-700">{title}</h3>
        <span
          className={`w-2 h-2 rounded-full ${connected ? "bg-green-400" : "bg-gray-300"}`}
        />
      </div>
      <ResponsiveContainer width="100%" height={200}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
          <XAxis
            dataKey="time"
            tick={{ fontSize: 11 }}
            tickLine={false}
            axisLine={false}
          />
          <YAxis
            tick={{ fontSize: 11 }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(v) => `${v}${unit}`}
          />
          <Tooltip
            contentStyle={{ fontSize: 12 }}
            formatter={(v: number) => [`${v}${unit}`, title]}
          />
          <Line
            type="monotone"
            dataKey="value"
            stroke={color}
            strokeWidth={2}
            dot={false}
            isAnimationActive={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
