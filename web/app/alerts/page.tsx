"use client";

import { useCallback, useEffect, useState } from "react";
import { get, type Alert } from "@/lib/api";

export default function AlertsPage() {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const data = await get<Alert[]>("/alerts");
      setAlerts(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "load failed");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!localStorage.getItem("pw_token")) {
      window.location.href = "/login";
      return;
    }
    load();
  }, [load]);

  if (loading) return <p className="text-sm text-zinc-500">Loading alerts...</p>;

  return (
    <div className="space-y-4">
      <h1 className="text-lg font-semibold">Alert log</h1>
      {error && <p className="text-xs text-red-400">{error}</p>}
      <div className="overflow-x-auto rounded-lg border border-zinc-900 bg-zinc-900">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-zinc-800 text-left text-xs text-zinc-500">
              <th className="px-4 py-2">Time</th>
              <th className="px-4 py-2">Kind</th>
              <th className="px-4 py-2">Target</th>
              <th className="px-4 py-2">Payload</th>
            </tr>
          </thead>
          <tbody>
            {alerts.map((a) => (
              <tr key={a.id} className="border-b border-zinc-800/50 last:border-0">
                <td className="px-4 py-2 font-mono-num text-xs text-zinc-400">
                  {new Date(a.created_at).toLocaleString()}
                </td>
                <td className="px-4 py-2 text-xs">{a.kind}</td>
                <td className="max-w-xs truncate px-4 py-2 font-mono-num text-xs text-zinc-400">{a.target}</td>
                <td className="max-w-md truncate px-4 py-2 font-mono-num text-xs text-zinc-500">{a.payload}</td>
              </tr>
            ))}
            {alerts.length === 0 && (
              <tr><td colSpan={4} className="px-4 py-6 text-center text-xs text-zinc-500">No alerts logged.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
