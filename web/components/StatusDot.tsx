export function StatusDot({ status }: { status?: string }) {
  const s = (status || "").toLowerCase();
  const color =
    s === "up" || s === "ok" || s === "healthy" || s === "resolved"
      ? "bg-green-500"
      : s === "down" || s === "fail" || s === "failed" || s === "open" || s === "critical"
        ? "bg-red-500"
        : s === "degraded" || s === "paused" || s === "pending" || s === "unknown" || s === ""
          ? "bg-amber-500"
          : "bg-amber-500";
  const pulse = color === "bg-red-500" || color === "bg-amber-500";
  return (
    <span className="relative inline-flex h-2.5 w-2.5">
      {pulse && (
        <span className={`pw-pulse-ring absolute inline-flex h-full w-full rounded-full ${color}`} />
      )}
      <span className={`relative inline-flex h-2.5 w-2.5 rounded-full ${color}`} />
    </span>
  );
}
