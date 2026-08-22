export interface Host {
  id: number;
  domain: string;
  upstream: string;
  request_headers: Record<string, string>;
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
  enabled: boolean;
}
