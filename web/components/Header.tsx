"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { logout } from "@/lib/api";

export function Header() {
  const router = useRouter();
  const [orgId, setOrgId] = useState<string | null>(null);
  const [authed, setAuthed] = useState(false);

  useEffect(() => {
    setOrgId(localStorage.getItem("pw_org_id"));
    setAuthed(!!localStorage.getItem("pw_token"));
  }, []);

  return (
    <header className="border-b border-zinc-900">
      <div className="mx-auto flex max-w-6xl items-center gap-4 px-4 py-3">
        <Link href="/monitors" className="flex items-center gap-2">
          <span className="relative inline-flex h-2.5 w-2.5">
            <span className="pw-pulse-ring absolute inline-flex h-full w-full rounded-full bg-rose-600" />
            <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-rose-600" />
          </span>
          <span className="text-sm font-semibold tracking-wide">PulseWatch</span>
        </Link>
        <nav className="flex items-center gap-4 text-sm text-zinc-400">
          <Link className="hover:text-zinc-100" href="/monitors">Monitors</Link>
          <Link className="hover:text-zinc-100" href="/incidents">Incidents</Link>
          <Link className="hover:text-zinc-100" href="/alerts">Alerts</Link>
          <Link className="hover:text-zinc-100" href="/status/demo">Status</Link>
        </nav>
        <div className="ml-auto flex items-center gap-3">
          {orgId && (
            <span className="rounded border border-zinc-800 px-2 py-1 font-mono-num text-xs text-zinc-400">
              org:{orgId.slice(0, 8)}
            </span>
          )}
          {authed && (
            <button
              className="rounded border border-zinc-800 px-2 py-1 text-xs text-zinc-400 hover:text-zinc-100"
              onClick={() => {
                logout();
                router.push("/login");
              }}
            >
              Logout
            </button>
          )}
        </div>
      </div>
    </header>
  );
}
