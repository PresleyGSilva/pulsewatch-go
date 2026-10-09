"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { login, register } from "@/lib/api";

const inputCls =
  "w-full rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 placeholder-zinc-600 outline-none focus:border-rose-700";

export default function LoginPage() {
  const router = useRouter();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("admin@pulsewatch.demo");
  const [password, setPassword] = useState("demo1234");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      if (mode === "login") {
        await login(email, password);
      } else {
        await register(name || email.split("@")[0], email, password);
      }
      router.push("/monitors");
    } catch (err) {
      setError(err instanceof Error ? err.message : "request failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto max-w-sm pt-16">
      <h1 className="text-xl font-semibold">Sign in to PulseWatch</h1>
      <p className="mt-1 text-xs text-zinc-500">
        Demo account: admin@pulsewatch.demo / demo1234
      </p>
      <form onSubmit={onSubmit} className="mt-6 space-y-3">
        {mode === "register" && (
          <input
            className={inputCls}
            placeholder="Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        )}
        <input
          className={inputCls}
          placeholder="Email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
        />
        <input
          className={inputCls}
          placeholder="Password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />
        {error && <p className="text-xs text-red-400">{error}</p>}
        <button
          type="submit"
          disabled={busy}
          className="w-full rounded-md bg-rose-700 px-4 py-2 text-sm font-medium text-white hover:bg-rose-600 disabled:opacity-50"
        >
          {busy ? "Please wait..." : mode === "login" ? "Sign in" : "Create account"}
        </button>
      </form>
      <p className="mt-4 text-xs text-zinc-500">
        {mode === "login" ? (
          <>
            No account?{" "}
            <button className="text-rose-400 hover:underline" onClick={() => setMode("register")}>
              Register
            </button>
          </>
        ) : (
          <>
            Have an account?{" "}
            <button className="text-rose-400 hover:underline" onClick={() => setMode("login")}>
              Sign in
            </button>
          </>
        )}
      </p>
      <p className="mt-6 text-xs text-zinc-600">
        Public status page: <Link className="text-rose-400 hover:underline" href="/status/demo">/status/demo</Link>
      </p>
    </div>
  );
}
