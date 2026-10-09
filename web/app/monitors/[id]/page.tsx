"use client";

import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { get, type Check, type Incident, type Monitor } from "@/lib/api";
import { Sparkline } from "@/components/Sparkline";
import { StatusDot } from "@/components/StatusDot";
import { UptimeBar } from "@/components/UptimeBar";

interface UptimeResp {
  uptime_pct?: number;
  uptime?: number;
  pct?: number;
}

export default function MonitorDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id;
  const [monitor, setMonitor] = useState<Monitor | null>(null);
  const [checks, setChecks] = useState<Check[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [uptime, setUptime] = useState<number | undefined>(undefined);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const m = await get<Monitor>(`/monitors/${id}`);
      setMonitor(m);
      const [c, u, inc] = await Promise.all([
        get<Check[]>(`/monitors/${id}/checks`).catch(() => [] as Check[]),
        get<UptimeResp>(`/monitors/${id}/uptime?hours=24`).catch(() => null),
        get<Incident[]>(`/monitors/${id}/incidents`).catch(() => [] as Incident[]),
      ]);
      setChecks(Array.isArray(c) ? c : []);
      if (u) setUptime(u.uptime_pct ?? u.uptime ?? u.pct ?? (m as Monitor).uptime_24h);
      else setUptime((m as Monitor).uptime_24h);
      setIncidents(Array.isArray(inc) ? inc : []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "load failed");
    }
  }, [id]);

  useEffect(() => {
    if (!localStorage.getItem("pw_token")) {
      window.location.href = "/login";
      return;
    }
    load();
  }, [load]);

  return (
    <div className="space-y-6">
      {error && <p className="text-xs text-red-400">{error}</p>}
      {!monitor ? (
        <p className="text-sm text-zinc-500">Loading...</p>
      ) : (
        <>
          <div className="flex items-center gap-3">
            <StatusDot status={monitor.active ? monitor.last_status || "unknown" : "paused"} />
            <h1 className="text-lg font-semibold">{monitor.name}</h1>
            <span className="font-mono-num text-xs text-zinc-500">{monitor.url}</span>
          </div>

          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            <div className="rounded-lg border border-zinc-900 bg-zinc-900 p-4">
              <p className="text-xs text-zinc-500">Uptime 24h</p>
              <div className="mt-2"><UptimeBar pct={uptime ?? monitor.uptime_24h} /></div>
            </div>
            <div className="rounded-lg border border-zinc-900 bg-zinc-900 p-4">
              <p className="text-xs text-zinc-500">Last check</p>
              <p className="font-mono-num mt-2 text-sm">
                {checks[0]?.status_code ?? monitor.last_code ?? "-"} · {checks[0]?.latency_ms ?? monitor.last_latency_ms ?? "-"} ms
              </p>
            </div>
            <div className="rounded-lg border border-zinc-900 bg-zinc-900 p-4">
              <p className="text-xs text-zinc-500">Config</p>
              <p className="font-mono-num mt-2 text-xs text-zinc-400">
                every {monitor.interval_seconds}s · timeout {monitor.timeout_ms}ms · expect {monitor.expect_status}
              </p>
            </div>
          </div>

          <div className="rounded-lg border border-zinc-900 bg-zinc-900 p-4">
            <h2 className="mb-2 text-sm font-semibold">Latency (ms)</h2>
            <Sparkline values={[...checks].reverse().map((c) => c.latency_ms)} />
          </div>

          <div className="rounded-lg border border-zinc-900 bg-zinc-900">
            <h2 className="border-b border-zinc-800 px-4 py-2 text-sm font-semibold">Recent checks</h2>
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-xs text-zinc-500">
                  <th className="px-4 py-2">Time</th>
                  <th className="px-4 py-2">Code</th>
                  <th className="px-4 py-2">OK</th>
                  <th className="px-4 py-2">Latency</th>
                  <th className="px-4 py-2">Error</th>
                </tr>
              </thead>
              <tbody>
                {checks.map((c) => (
                  <tr key={c.id} className="border-t border-zinc-800/50">
                    <td className="px-4 py-2 font-mono-num text-xs text-zinc-400">
                      {new Date(c.checked_at ?? c.created_at ?? "").toLocaleString()}
                    </td>
                    <td className="px-4 py-2 font-mono-num text-xs">{c.status_code}</td>
                    <td className="px-4 py-2">
                      <StatusDot status={c.ok ? "up" : "down"} />
                    </td>
                    <td className="px-4 py-2 font-mono-num text-xs">{c.latency_ms} ms</td>
                    <td className="max-w-xs truncate px-4 py-2 text-xs text-red-400">{c.error || "-"}</td>
                  </tr>
                ))}
                {checks.length === 0 && (
                  <tr><td colSpan={5} className="px-4 py-4 text-center text-xs text-zinc-500">No checks yet.</td></tr>
                )}
              </tbody>
            </table>
          </div>

          <div className="rounded-lg border border-zinc-900 bg-zinc-900">
            <h2 className="border-b border-zinc-800 px-4 py-2 text-sm font-semibold">Incidents</h2>
            {incidents.length === 0 ? (
              <p className="px-4 py-4 text-xs text-zinc-500">No incidents for this monitor.</p>
            ) : (
              <ul className="divide-y divide-zinc-800/50">
                {incidents.map((i) => (
                  <li key={i.id} className="flex items-center gap-3 px-4 py-2 text-sm">
                    <StatusDot status={i.status} />
                    <span className="font-mono-num text-xs text-zinc-400">{new Date(i.started_at).toLocaleString()}</span>
                    <span className="text-xs">{i.message || i.status}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </>
      )}
    </div>
  );
}
