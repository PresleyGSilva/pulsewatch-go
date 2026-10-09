export function Sparkline({
  values,
  width = 320,
  height = 64,
}: {
  values: number[];
  width?: number;
  height?: number;
}) {
  if (!values || values.length === 0) {
    return <div className="text-xs text-zinc-500">No data</div>;
  }
  const max = Math.max(...values, 1);
  const min = Math.min(...values, 0);
  const span = max - min || 1;
  const step = values.length > 1 ? width / (values.length - 1) : 0;
  const pts = values
    .map((v, i) => {
      const x = values.length > 1 ? i * step : width / 2;
      const y = height - 4 - ((v - min) / span) * (height - 8);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
  return (
    <svg width={width} height={height} className="overflow-visible">
      <polyline
        points={pts}
        fill="none"
        stroke="#E11D48"
        strokeWidth="1.5"
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  );
}
