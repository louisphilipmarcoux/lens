import { useState } from "react";
import MetricChart from "../components/MetricChart";
import TemplateVars from "../components/TemplateVars";
import type { TemplateVariable } from "../types";

const defaultVars: TemplateVariable[] = [
  {
    name: "host",
    label: "Host",
    values: ["web-01", "web-02", "db-01"],
    selected: "",
  },
  {
    name: "service",
    label: "Service",
    values: ["api", "worker", "frontend"],
    selected: "",
  },
];

export default function Dashboard() {
  const [vars, setVars] = useState(defaultVars);

  const handleVarChange = (name: string, value: string) => {
    setVars((prev) =>
      prev.map((v) => (v.name === name ? { ...v, selected: value } : v))
    );
  };

  const hostFilter = vars.find((v) => v.name === "host")?.selected;
  const hostQuery = hostFilter ? `{host="${hostFilter}"}` : "";

  return (
    <div>
      <h2 className="text-lg font-semibold text-gray-800 mb-4">
        System Dashboard
      </h2>

      <TemplateVars variables={vars} onChange={handleVarChange} />

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        <MetricChart
          title="CPU Usage"
          query={`cpu.user_percent${hostQuery}`}
          color="#ef4444"
          unit="%"
        />
        <MetricChart
          title="Memory Usage"
          query={`memory.used_percent${hostQuery}`}
          color="#8b5cf6"
          unit="%"
        />
        <MetricChart
          title="Load Average (1m)"
          query={`loadavg.1min${hostQuery}`}
          color="#f59e0b"
        />
        <MetricChart
          title="Network RX"
          query={`net.rx_bytes_per_sec${hostQuery}`}
          color="#06b6d4"
          unit=" B/s"
        />
        <MetricChart
          title="Network TX"
          query={`net.tx_bytes_per_sec${hostQuery}`}
          color="#10b981"
          unit=" B/s"
        />
        <MetricChart
          title="Disk Read"
          query={`disk.read_bytes_per_sec${hostQuery}`}
          color="#6366f1"
          unit=" B/s"
        />
      </div>
    </div>
  );
}
