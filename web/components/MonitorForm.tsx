"use client";

import { useState } from "react";

export interface MonitorFormValues {
  name: string;
  url: string;
  interval_seconds: number;
  timeout_ms: number;
  expect_status: number;
  expect_body_contains: string;
  webhook_url: string;
  active: boolean;
}

const inputCls =
  "w-full rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 placeholder-zinc-600 outline-none focus:border-rose-700";

export function MonitorForm({
  onSubmit,
  submitting,
}: {
  onSubmit: (v: MonitorFormValues) => Promise<void> | void;
  submitting: boolean;
}) {
  const [form, setForm] = useState<MonitorFormValues>({
    name: "",
    url: "https://",
    interval_seconds: 60,
    timeout_ms: 5000,
    expect_status: 200,
    expect_body_contains: "",
    webhook_url: "",
    active: true,
  });

  function set<K extends keyof MonitorFormValues>(k: K, v: MonitorFormValues[K]) {
    setForm((f) => ({ ...f, [k]: v }));
  }

  return (
    <form
      className="grid grid-cols-1 gap-3 md:grid-cols-2"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(form);
      }}
    >
      <label className="text-xs text-zinc-400">
        Name
        <input className={inputCls} value={form.name} onChange={(e) => set("name", e.target.value)} required placeholder="API" />
      </label>
      <label className="text-xs text-zinc-400">
        URL
        <input className={inputCls} value={form.url} onChange={(e) => set("url", e.target.value)} required placeholder="https://example.com/health" />
      </label>
      <label className="text-xs text-zinc-400">
        Interval (seconds)
        <input
          className={inputCls}
          type="number"
          min={10}
          value={form.interval_seconds}
          onChange={(e) => set("interval_seconds", Number(e.target.value))}
        />
      </label>
      <label className="text-xs text-zinc-400">
        Timeout (ms)
        <input
          className={inputCls}
          type="number"
          min={1000}
          value={form.timeout_ms}
          onChange={(e) => set("timeout_ms", Number(e.target.value))}
        />
      </label>
      <label className="text-xs text-zinc-400">
        Expected status
        <input
          className={inputCls}
          type="number"
          value={form.expect_status}
          onChange={(e) => set("expect_status", Number(e.target.value))}
        />
      </label>
      <label className="text-xs text-zinc-400">
        Body must contain (optional)
        <input
          className={inputCls}
          value={form.expect_body_contains}
          onChange={(e) => set("expect_body_contains", e.target.value)}
          placeholder="ok"
        />
      </label>
      <label className="text-xs text-zinc-400 md:col-span-2">
        Webhook URL (optional)
        <input
          className={inputCls}
          value={form.webhook_url}
          onChange={(e) => set("webhook_url", e.target.value)}
          placeholder="https://hooks.example.com/..."
        />
      </label>
      <label className="flex items-center gap-2 text-xs text-zinc-400">
        <input type="checkbox" checked={form.active} onChange={(e) => set("active", e.target.checked)} />
        Active
      </label>
      <div className="md:col-span-2">
        <button
          type="submit"
          disabled={submitting}
          className="rounded-md bg-rose-700 px-4 py-2 text-sm font-medium text-white hover:bg-rose-600 disabled:opacity-50"
        >
          {submitting ? "Saving..." : "Create monitor"}
        </button>
      </div>
    </form>
  );
}
