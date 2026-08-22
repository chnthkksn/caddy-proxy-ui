import type {
  DeleteResult,
  Host,
  HostInput,
  HostMutationResult,
  ImportResult,
  StatusResponse,
  SyncStatus,
} from "./types";

const BASE = "/api";

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method,
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    credentials: "same-origin",
  });

  const isJson = res.headers.get("content-type")?.includes("application/json") ?? false;
  const data = isJson ? await res.json() : await res.text();

  if (!res.ok) {
    const message = isJson && data && typeof data === "object" && "error" in data
      ? String((data as { error: unknown }).error)
      : `Request failed (${res.status})`;
    throw new Error(message);
  }
  return data as T;
}

export const api = {
  status: () => request<StatusResponse>("GET", "/status"),
  setup: (username: string, password: string, confirmPassword: string) =>
    request<{ ok: boolean }>("POST", "/setup", {
      username,
      password,
      confirm_password: confirmPassword,
    }),
  login: (username: string, password: string) =>
    request<{ ok: boolean }>("POST", "/login", { username, password }),
  logout: () => request<{ ok: boolean }>("POST", "/logout"),

  listHosts: () => request<Host[]>("GET", "/hosts"),
  createHost: (host: HostInput) => request<HostMutationResult>("POST", "/hosts", host),
  updateHost: (id: number, host: HostInput) =>
    request<HostMutationResult>("PUT", `/hosts/${id}`, host),
  deleteHost: (id: number) => request<DeleteResult>("DELETE", `/hosts/${id}`),
  toggleHost: (id: number) => request<HostMutationResult>("POST", `/hosts/${id}/toggle`),

  sync: () => request<SyncStatus>("POST", "/sync"),
  importCaddyfile: (caddyfile: string) =>
    request<ImportResult>("POST", "/import", { caddyfile }),
};
