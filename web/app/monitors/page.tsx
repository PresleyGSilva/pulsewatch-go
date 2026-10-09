"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { del, get, post, put, type Monitor } from "@/lib/api";
import { MonitorForm, type MonitorFormValues } from "@/components/MonitorForm";
import { StatusDot } from "@/components/StatusDot";
import { UptimeBar } from "@/components/UptimeBar";

export default function MonitorsPage() {
  const router = useRouter();
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [checking, setChecking] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await get<Monitor[]>("/monitors");
      setMonitors(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "load failed");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!localStorage.getItem("pw_token")) {
      router.replace("/login");
      return;
    }
    load();
  }, [load, router]);

  async function create(v: MonitorFormValues) {
    setCreating(true);
    setError("");
    try {
      const payload = {
        name: v.name,
        url: v.url,
        interval_seconds: v.interval_seconds,
        timeout_ms: v.timeout_ms,
        expect_status: v.expect_status,
        expect_body_contains: v.expect_body_contains || undefined,
        webhook_url: v.webhook_url || undefined,
        active: v.active,
      };
      await post<Monitor>("/monitors", payload);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "create failed");
    } finally {
      setCreating(false);
    }
  }

  async function togglePause(m: Monitor) {
    try {
      await put(`/monitors/${m.id}`, { active: !m.active });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "update failed");
    }
  }

  async function remove(id: string) {
    if (!confirm("Delete this monitor?")) return;
    try {
      await del(`/monitors/${id}`);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "delete failed");
    }
  }

  async function checkNow(id: string) {
    setChecking(id);
    try {
      await post(`/monitors/${id}/check`, {});
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "check failed");
    } finally {
      setChecking(null);
    }
  }

  if (loading) return <p className="text-sm text-zinc-500">Loading monitors...</p>;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">Monitors</h1>
        <span className="font-mono-num text-xs text-zinc-500">{monitors.length} total</span>
      </div>
      {error && <p className="text-xs text-red-400">{error}</p>}

      <div className="overflow-x-auto rounded-lg border border-zinc-900 bg-zinc-900">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-zinc-800 text-left text-xs text-zinc-500">
              <th className="px-4 py-2">Status</th>
              <th className="px-4 py-2">Name</th>
              <th className="px-4 py-2">URL</th>
              <th className="px-4 py-2">Interval</th>
              <th className="px-4 py-2">Uptime 24h</th>
              <th className="px-4 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {monitors.map((m) => (
              <tr key={m.id} className="border-b border-zinc-800/50 last:border-0">
                <td className="px-4 py-2">
                  <StatusDot status={m.active ? m.last_status || "unknown" : "paused"} />
                </td>
                <td className="px-4 py-2">
                  <Link href={`/monitors/${m.id}`} className="font-medium text-zinc-100 hover:text-rose-400">
                    {m.name}
                  </Link>
                </td>
                <td className="max-w-xs truncate px-4 py-2 font-mono-num text-xs text-zinc-400">{m.url}</td>
                <td className="px-4 py-2 font-mono-num text-xs text-zinc-400">{m.interval_seconds}s</td>
                <td className="px-4 py-2">
                  <Link href={`/monitors/${m.id}`} className="hover:underline">
                    <UptimeBar pct={m.uptime_24h} />
                  </Link>
                </td>
                <td className="px-4 py-2">
                  <div className="flex gap-2 text-xs">
                    <button onClick={() => checkNow(m.id)} disabled={checking === m.id} className="text-zinc-300 hover:text-white disabled:opacity-50">
                      {checking === m.id ? "..." : "Check now"}
                    </button>
                    <button onClick={() => togglePause(m)} className="text-zinc-400 hover:text-white">
                      {m.active ? "Pause" : "Resume"}
                    </button>
                    <button onClick={() => remove(m.id)} className="text-red-400 hover:text-red-300">
                      Delete
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {monitors.length === 0 && (
              <tr>
                <td colSpan={6} className="px-4 py-6 text-center text-xs text-zinc-500">
                  No monitors yet. Create one below.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="rounded-lg border border-zinc-900 bg-zinc-900 p-4">
        <h2 className="mb-3 text-sm font-semibold">New monitor</h2>
        <MonitorForm onSubmit={create} submitting={creating} />
      </div>
    </div>
  );
}
