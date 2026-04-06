import { Routes, Route, Navigate } from "react-router-dom";
import Layout from "./components/Layout";
import Dashboard from "./pages/Dashboard";
import LogExplorer from "./pages/LogExplorer";
import TraceExplorer from "./pages/TraceExplorer";
import TraceDetail from "./pages/TraceDetail";

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/logs" element={<LogExplorer />} />
        <Route path="/traces" element={<TraceExplorer />} />
        <Route path="/traces/:traceId" element={<TraceDetail />} />
      </Routes>
    </Layout>
  );
}
