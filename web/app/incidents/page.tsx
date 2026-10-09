"use client";

import { useCallback, useEffect, useState } from "react";
import { get, post, type Incident } from "@/lib/api";
import { StatusDot } from "@/components/StatusDot";

export default function IncidentsPage() {
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [filter, setFilter] = useState<"all" | "open" | "resolved">("all");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const data = await get<Incident[]>("/incidents");
      setIncidents(Array.isArray(data) ? data : []);
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

  async function resolve(id: string) {
    try {
      await post(`/incidents/${id}/resolve`, {});
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "resolve failed");
    }
  }

  const shown = incidents.filter((i) => {
    if (filter === "all") return true;
    const open = i.status.toLowerCase() !== "resolved";
    return filter === "open" ? open : !open;
  });

  if (loading) return <p className="text-sm text-zinc-500">Loading incidents...</p>;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">Incidents</h1>
        <div className="flex gap-2 text-xs">
          {(["all", "open", "resolved"] as const).map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className={`rounded border px-2 py-1 capitalize ${filter === f ? "border-rose-700 text-rose-300" : "border-zinc-800 text-zinc-400"}`}
            >
              {f}
            </button>
          ))}
        </div>
      </div>
      {error && <p className="text-xs text-red-400">{error}</p>}
      <div className="overflow-x-auto rounded-lg border border-zinc-900 bg-zinc-900">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-zinc-800 text-left text-xs text-zinc-500">
              <th className="px-4 py-2">Status</th>
              <th className="px-4 py-2">Monitor</th>
              <th className="px-4 py-2">Started</th>
              <th className="px-4 py-2">Message</th>
              <th className="px-4 py-2">Action</th>
            </tr>
          </thead>
          <tbody>
            {shown.map((i) => {
              const open = i.status.toLowerCase() !== "resolved";
              return (
                <tr key={i.id} className="border-b border-zinc-800/50 last:border-0">
                  <td className="px-4 py-2">
                    <span className="flex items-center gap-2 text-xs">
                      <StatusDot status={i.status} /> {i.status}
                    </span>
                  </td>
                  <td className="px-4 py-2 text-xs">{i.monitor_name || i.monitor_id}</td>
                  <td className="px-4 py-2 font-mono-num text-xs text-zinc-400">
                    {new Date(i.started_at).toLocaleString()}
                  </td>
                  <td className="px-4 py-2 text-xs text-zinc-400">{i.message || "-"}</td>
                  <td className="px-4 py-2">
                    {open && (
                      <button onClick={() => resolve(i.id)} className="text-xs text-rose-400 hover:underline">
                        Resolve
                      </button>
                    )}
                  </td>
                </tr>
              );
            })}
            {shown.length === 0 && (
              <tr><td colSpan={5} className="px-4 py-6 text-center text-xs text-zinc-500">No incidents.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
