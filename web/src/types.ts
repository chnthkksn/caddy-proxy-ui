export interface Host {
  id: number;
  domain: string;
  upstream: string;
  request_headers: Record<string, string>;
  group_label: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface SyncStatus {
  synced: boolean;
  error?: string;
}

export interface HostMutationResult {
  host: Host;
  sync: SyncStatus;
}

export interface DeleteResult {
  sync: SyncStatus;
}

export interface StatusResponse {
  setup_required: boolean;
  caddy_connected: boolean;
  host_count: number;
  version: string;
}

export interface SkippedHost {
  domain: string;
  reason: string;
}

export interface ImportResult {
  imported: number;
  skipped: SkippedHost[];
  sync: SyncStatus;
}

export interface HostInput {
  domain: string;
  upstream: string;
  request_headers: Record<string, string>;
  group_label: string;
  enabled: boolean;
}

export type AccessRuleKind = "basic_auth" | "ip_allow" | "ip_deny";

export interface AccessRule {
  id: number;
  host_id: number;
  kind: AccessRuleKind;
  value: string; // basic_auth: username only, never the hash; ip_*: the CIDR/IP
  created_at: string;
}

export interface AccessRuleMutationResult {
  rule: AccessRule;
  sync: SyncStatus;
}

export interface HourBucket {
  hour: string; // RFC3339, start of the hour, UTC
  requests: number;
  errors: number;
}

export interface LogEntry {
  time: string; // RFC3339
  domain: string;
  status: number;
  duration: number; // seconds
}

export interface TrafficSnapshot {
  available: boolean;
  hourly: HourBucket[];
  recent: LogEntry[];
  total_24h: number;
  errors_24h: number;
  by_domain: Record<string, number>;
}

export interface CertificateInfo {
  domain: string;
  issuer: string;
  not_before: string; // RFC3339
  not_after: string; // RFC3339
}

export interface CertificatesResponse {
  available: boolean;
  certificates: CertificateInfo[];
}

export interface InstanceSettings {
  caddy_admin_url: string;
  access_log_path: string;
  cert_storage_path: string;
  version: string;
}

export interface Overview {
  host_count: number;
  live_host_count: number;

  certs_available: boolean;
  cert_count: number;
  earliest_expiring_domain?: string;
  earliest_expiring_at?: string; // RFC3339

  traffic_available: boolean;
  requests_24h: number;
  errors_24h: number;

  footprint_bytes: number;
}
