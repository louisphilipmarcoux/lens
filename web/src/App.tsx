import { Navigate, useLocation } from "react-router-dom";
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

  if (pathname === "/") return <Navigate to="/dashboard" replace />;

  // Match /traces/<traceId> for detail view.
  const traceMatch = pathname.match(/^\/traces\/(.+)$/);
  const traceId = traceMatch ? traceMatch[1] : null;

  return (
    <Layout>
      <KeepAlive visible={pathname === "/dashboard"}>
        <Dashboard />
      </KeepAlive>
      <KeepAlive visible={pathname === "/logs"}>
        <LogExplorer />
      </KeepAlive>
      <KeepAlive visible={pathname === "/traces"}>
        <TraceExplorer />
      </KeepAlive>
      {traceId && <TraceDetail traceId={traceId} />}
    </Layout>
  );
}
