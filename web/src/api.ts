import type {
  AccessRule,
  AccessRuleMutationResult,
  CertificatesResponse,
  DeleteResult,
  Host,
  HostInput,
  HostMutationResult,
  ImportResult,
  InstanceSettings,
  LoginResponse,
  Overview,
  StatusResponse,
  SyncStatus,
  TOTPSetup,
  TrafficSnapshot,
} from "./types";

const BASE = "/api";

// ApiError carries the HTTP status so callers can tell e.g. an expired
// two-factor challenge (410) from a wrong code (401).
export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

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
    throw new ApiError(message, res.status);
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
    request<LoginResponse>("POST", "/login", { username, password }),
  loginTOTP: (challenge: string, code: string) =>
    request<LoginResponse>("POST", "/login/totp", { challenge, code }),
  logout: () => request<{ ok: boolean }>("POST", "/logout"),
  changePassword: (currentPassword: string, newPassword: string, confirmPassword: string) =>
    request<{ ok: boolean }>("POST", "/change-password", {
      current_password: currentPassword,
      new_password: newPassword,
      confirm_password: confirmPassword,
    }),
  totpSetup: (password: string) => request<TOTPSetup>("POST", "/totp/setup", { password }),
  totpEnable: (code: string) => request<{ ok: boolean }>("POST", "/totp/enable", { code }),
  totpDisable: (password: string, code: string) =>
    request<{ ok: boolean }>("POST", "/totp/disable", { password, code }),

  listHosts: () => request<Host[]>("GET", "/hosts"),
  createHost: (host: HostInput) => request<HostMutationResult>("POST", "/hosts", host),
  updateHost: (id: number, host: HostInput) =>
    request<HostMutationResult>("PUT", `/hosts/${id}`, host),
  deleteHost: (id: number) => request<DeleteResult>("DELETE", `/hosts/${id}`),
  toggleHost: (id: number) => request<HostMutationResult>("POST", `/hosts/${id}/toggle`),

  sync: () => request<SyncStatus>("POST", "/sync"),
  importCaddyfile: (caddyfile: string) =>
    request<ImportResult>("POST", "/import", { caddyfile }),
  exportCaddyfile: () => request<string>("GET", "/export"),

  listAccessRules: (hostId: number) =>
    request<AccessRule[]>("GET", `/hosts/${hostId}/access-rules`),
  addBasicAuthRule: (hostId: number, username: string, password: string) =>
    request<AccessRuleMutationResult>("POST", `/hosts/${hostId}/access-rules`, {
      kind: "basic_auth",
      username,
      password,
    }),
  addIPRule: (hostId: number, kind: "ip_allow" | "ip_deny", value: string) =>
    request<AccessRuleMutationResult>("POST", `/hosts/${hostId}/access-rules`, {
      kind,
      value,
    }),
  deleteAccessRule: (id: number) => request<DeleteResult>("DELETE", `/access-rules/${id}`),

  traffic: () => request<TrafficSnapshot>("GET", "/traffic"),
  certificates: () => request<CertificatesResponse>("GET", "/certificates"),
  overview: () => request<Overview>("GET", "/overview"),
  settings: () => request<InstanceSettings>("GET", "/settings"),
};
