export function UptimeBar({ pct }: { pct?: number }) {
  const v = pct ?? 0;
  const color = v >= 99 ? "bg-green-500" : v >= 95 ? "bg-amber-500" : "bg-red-500";
  return (
    <div className="flex items-center gap-2">
      <div className="h-1.5 w-24 overflow-hidden rounded-full bg-zinc-800">
        <div className={`h-full rounded-full ${color}`} style={{ width: `${Math.min(100, Math.max(0, v))}%` }} />
      </div>
      <span className="font-mono-num text-xs text-zinc-400">{v.toFixed(2)}%</span>
    </div>
  );
}
