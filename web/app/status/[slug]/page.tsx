import { API_BASE } from "@/lib/api";
import { StatusDot } from "@/components/StatusDot";
import { UptimeBar } from "@/components/UptimeBar";

interface PublicMonitor {
  id: string;
  name: string;
  url?: string;
  last_status?: string;
  uptime_24h?: number;
}

interface PublicIncident {
  id: string;
  monitor_name?: string;
  status: string;
  started_at: string;
  message?: string;
}

interface PublicStatus {
  name?: string;
  org?: { name?: string; slug?: string };
  status?: string;
  monitors?: PublicMonitor[];
  incidents?: PublicIncident[];
  open_incidents?: PublicIncident[];
}

async function fetchStatus(slug: string): Promise<PublicStatus | null> {
  try {
    const res = await fetch(`${API_BASE}/public/${slug}/status`, { cache: "no-store" });
    if (!res.ok) return null;
    const raw: unknown = await res.json();
    if (raw !== null && typeof raw === "object" && "data" in raw) {
      const data: unknown = raw.data;
      return data as PublicStatus; // boundary: API envelope {data} unwrapped once here
    }
    return raw as PublicStatus; // boundary: already-unwrapped status payload
  } catch {
    return null;
  }
}

export default async function StatusPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const data = await fetchStatus(slug);
  const open = data?.open_incidents ?? data?.incidents?.filter((i) => i.status.toLowerCase() !== "resolved") ?? [];
  const degraded = open.length > 0;
  const monitors = data?.monitors ?? [];

  return (
    <div className="mx-auto max-w-3xl space-y-6 pt-6">
      <div
        className={`rounded-lg border p-6 text-center ${degraded ? "border-amber-800 bg-amber-950/30" : "border-green-900 bg-green-950/30"}`}
      >
        <div className="flex items-center justify-center gap-3">
          <StatusDot status={data ? (degraded ? "degraded" : "up") : "unknown"} />
          <h1 className="text-xl font-semibold">
            {!data
              ? "Status unavailable"
              : degraded
                ? "Degraded performance"
                : "All systems operational"}
          </h1>
        </div>
        <p className="mt-1 text-xs text-zinc-500">
          {data?.org?.name || data?.name || slug} · updated just now
        </p>
      </div>

      {!data ? (
        <p className="text-center text-sm text-zinc-500">
          Could not load status for slug &quot;{slug}&quot;. Try <span className="font-mono-num">/status/demo</span>.
        </p>
      ) : (
        <>
          <div className="space-y-3">
            {monitors.map((m) => (
              <div key={m.id} className="flex items-center gap-3 rounded-lg border border-zinc-900 bg-zinc-900 px-4 py-3">
                <StatusDot status={m.last_status || "unknown"} />
                <span className="text-sm font-medium">{m.name}</span>
                <span className="ml-auto"><UptimeBar pct={m.uptime_24h} /></span>
              </div>
            ))}
            {monitors.length === 0 && (
              <p className="text-center text-xs text-zinc-500">No monitors published to this page.</p>
            )}
          </div>

          {open.length > 0 && (
            <div className="rounded-lg border border-zinc-900 bg-zinc-900 p-4">
              <h2 className="mb-2 text-sm font-semibold">Open incidents</h2>
              <ul className="space-y-2">
                {open.map((i) => (
                  <li key={i.id} className="flex items-center gap-3 text-sm">
                    <StatusDot status={i.status} />
                    <span className="text-xs">{i.monitor_name}</span>
                    <span className="font-mono-num text-xs text-zinc-500">
                      {new Date(i.started_at).toLocaleString()}
                    </span>
                    <span className="text-xs text-zinc-400">{i.message}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </>
      )}
    </div>
  );
}
