import { api } from "./api";
import { errorMessage } from "./errors";
import type {
  CertificatesResponse,
  Host,
  Overview,
  StatusResponse,
  SyncStatus,
  TrafficSnapshot,
} from "./types";

export type Page =
  | "overview"
  | "hosts"
  | "edit"
  | "certificates"
  | "access"
  | "logs"
  | "caddyfile"
  | "settings";

/** Shared app data. One fetch per resource, shared by every page, instead of
 *  each page loading its own copy of the host list. */
class AppData {
  hosts = $state<Host[]>([]);
  traffic = $state<TrafficSnapshot | null>(null);
  certs = $state<CertificatesResponse | null>(null);
  overview = $state<Overview | null>(null);
  status = $state<StatusResponse>({
    setup_required: false,
    caddy_connected: false,
    host_count: 0,
    version: "",
  });

  error = $state("");
  /** Set when a write succeeded in SQLite but couldn't reach Caddy. */
  syncWarning = $state("");

  hostCount = $derived(this.hosts.length);
  liveCount = $derived(this.hosts.filter((h) => h.enabled).length);

  /** Real per-host request counts for the last 24h, keyed by domain. */
  reqsFor(domain: string): number | null {
    if (!this.traffic?.available) return null;
    return this.traffic.by_domain[domain] ?? 0;
  }

  /** The real certificate for a host, if Caddy has issued one. */
  certFor(domain: string) {
    if (!this.certs?.available) return null;
    return this.certs.certificates.find((c) => c.domain === domain) ?? null;
  }

  async refreshStatus() {
    try {
      this.status = await api.status();
    } catch {
      // Transient — keep the last known status rather than flapping the UI.
    }
  }

  async loadHosts() {
    this.hosts = await api.listHosts();
  }

  async loadAll() {
    this.error = "";
    const results = await Promise.allSettled([
      api.listHosts(),
      api.traffic(),
      api.certificates(),
      api.overview(),
      api.status(),
    ]);

    if (results[0].status === "fulfilled") this.hosts = results[0].value;
    else this.error = errorMessage(results[0].reason);

    if (results[1].status === "fulfilled") this.traffic = results[1].value;
    if (results[2].status === "fulfilled") this.certs = results[2].value;
    if (results[3].status === "fulfilled") this.overview = results[3].value;
    if (results[4].status === "fulfilled") this.status = results[4].value;
  }

  noteSync(sync: SyncStatus) {
    this.syncWarning = sync.synced
      ? ""
      : sync.error || "Caddy didn't accept the config push.";
  }

  clear() {
    this.hosts = [];
    this.traffic = null;
    this.certs = null;
    this.overview = null;
    this.error = "";
    this.syncWarning = "";
  }
}

export const data = new AppData();

/** Theme is a viewer preference, so it lives in localStorage, not the DB. */
class Theme {
  value = $state<"light" | "dark">(readStoredTheme());

  toggle() {
    this.value = this.value === "dark" ? "light" : "dark";
    try {
      localStorage.setItem("caddy-ui-theme", this.value);
    } catch {
      // Private mode / storage disabled — the toggle still works for this
      // session, it just won't be remembered.
    }
  }
}

function readStoredTheme(): "light" | "dark" {
  try {
    const stored = localStorage.getItem("caddy-ui-theme");
    if (stored === "dark" || stored === "light") return stored;
  } catch {
    // ignore
  }
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export const theme = new Theme();

// ── shared formatting ───────────────────────────────────────────────────

export function formatCount(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${Math.round(n / 100) / 10}k`;
  return `${Math.round(n / 100_000) / 10}M`;
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${Math.round((bytes / (1024 * 1024)) * 10) / 10} MB`;
}

export function daysUntil(iso: string): number {
  return Math.ceil((new Date(iso).getTime() - Date.now()) / 86_400_000);
}

/** "in 61 days" / "in 3 days" / "today" / "expired" — the design's phrasing. */
export function renewalPhrase(iso: string): string {
  const days = daysUntil(iso);
  if (days < 0) return "expired";
  if (days === 0) return "today";
  if (days === 1) return "tomorrow";
  return `in ${days} days`;
}

export function statusClass(status: number): string {
  // Caddy logs status 0 when the client went away before it wrote a
  // response. That isn't a success, so it must not render as one.
  if (status < 100) return "st-none";
  if (status >= 500) return "st-5xx";
  if (status >= 400) return "st-4xx";
  if (status >= 300) return "st-3xx";
  return "st-2xx";
}

/** What to print for a status code — 0 has no meaning to a reader. */
export function statusLabel(status: number): string {
  return status < 100 ? "—" : String(status);
}
