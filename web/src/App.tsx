import { Routes, Route, Navigate, useLocation } from "react-router-dom";
import Layout from "./components/Layout";
import Dashboard from "./pages/Dashboard";
import LogExplorer from "./pages/LogExplorer";
import TraceExplorer from "./pages/TraceExplorer";
import TraceDetail from "./pages/TraceDetail";

function KeepAlive({ visible, children }: { visible: boolean; children: React.ReactNode }) {
  return <div style={{ display: visible ? "block" : "none" }}>{children}</div>;
}

export default function App() {
  const { pathname } = useLocation();

  return (
    <Layout>
      {/* Main pages stay mounted — hidden via CSS, never unmounted */}
      <KeepAlive visible={pathname === "/dashboard" || pathname === "/"}>
        <Dashboard />
      </KeepAlive>
      <KeepAlive visible={pathname === "/logs"}>
        <LogExplorer />
      </KeepAlive>
      <KeepAlive visible={pathname === "/traces"}>
        <TraceExplorer />
      </KeepAlive>

      {/* Trace detail is dynamic — uses normal routing */}
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/traces/:traceId" element={<TraceDetail />} />
        <Route path="*" element={null} />
      </Routes>
    </Layout>
  );
}
