export const API_BASE =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8086";

export interface Org {
  id: string;
  name: string;
  slug: string;
}

export interface User {
  id: string;
  email: string;
  name?: string;
}

export interface Monitor {
  id: string;
  name: string;
  url: string;
  interval_seconds: number;
  timeout_ms: number;
  expect_status: number;
  expect_body_contains?: string;
  webhook_url?: string;
  active: boolean;
  last_status?: string;
  last_code?: number;
  last_latency_ms?: number;
  last_checked_at?: string;
  uptime_24h?: number;
}

export interface Check {
  id: string;
  monitor_id: string;
  status_code: number;
  ok: boolean;
  latency_ms: number;
  error?: string;
  checked_at?: string;
  created_at?: string;
}

export interface Incident {
  id: string;
  monitor_id: string;
  monitor_name?: string;
  status: string;
  started_at: string;
  resolved_at?: string;
  message?: string;
}

export interface Alert {
  id: string;
  kind: string;
  target: string;
  payload: string;
  created_at: string;
}

function storage(): Storage | null {
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

export function getToken(): string | null {
  return storage()?.getItem("pw_token") ?? null;
}

export function getOrgId(): string | null {
  return storage()?.getItem("pw_org_id") ?? null;
}

export function getUserId(): string | null {
  return storage()?.getItem("pw_user") ?? null;
}

export function isAuthed(): boolean {
  return !!getToken();
}

function headers(extra: Record<string, string> = {}): Record<string, string> {
  const h: Record<string, string> = { "Content-Type": "application/json", ...extra };
  const token = getToken();
  const orgId = getOrgId();
  const userId = getUserId();
  if (token) h["Authorization"] = `Bearer ${token}`;
  if (orgId) h["X-Org-ID"] = orgId;
  if (userId) h["X-User"] = userId;
  return h;
}

async function handle<T>(res: Response): Promise<T> {
  if (res.status === 401 && typeof window !== "undefined") {
    window.location.href = "/login";
    throw new Error("unauthorized");
  }
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(text || `request failed: ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  const body: string = await res.text();
  if (!body) return undefined as T;
  const parsed: unknown = JSON.parse(body);
  if (parsed !== null && typeof parsed === "object" && "data" in parsed) {
    const data: unknown = parsed.data;
    return data as T; // boundary: API envelope {data} unwrapped once here
  }
  return parsed as T; // boundary: already-unwrapped payloads (public status uses res.json directly)
}

export async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, { headers: headers() });
  return handle<T>(res);
}

export async function post<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: headers(),
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  return handle<T>(res);
}

export async function put<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "PUT",
    headers: headers(),
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  return handle<T>(res);
}

export async function del<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "DELETE",
    headers: headers(),
  });
  return handle<T>(res);
}

interface AuthResponse {
  user: User;
  orgs: Org[];
  access_token: string;
}

function persistAuth(data: AuthResponse) {
  const s = storage();
  if (!s) return;
  s.setItem("pw_token", data.access_token);
  s.setItem("pw_user", data.user.id);
  s.setItem("pw_orgs", JSON.stringify(data.orgs ?? []));
  if (data.orgs && data.orgs.length > 0) {
    s.setItem("pw_org_id", data.orgs[0].id);
  }
}

export async function login(email: string, password: string): Promise<AuthResponse> {
  const res = await fetch(`${API_BASE}/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  const data = await handle<AuthResponse>(res);
  persistAuth(data);
  return data;
}

export async function register(
  name: string,
  email: string,
  password: string
): Promise<AuthResponse> {
  const res = await fetch(`${API_BASE}/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, email, password }),
  });
  const data = await handle<AuthResponse>(res);
  persistAuth(data);
  return data;
}

export function logout() {
  const s = storage();
  if (!s) return;
  s.removeItem("pw_token");
  s.removeItem("pw_org_id");
  s.removeItem("pw_orgs");
  s.removeItem("pw_user");
}
