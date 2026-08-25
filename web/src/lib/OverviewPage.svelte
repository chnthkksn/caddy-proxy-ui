<script lang="ts">
  import { data, formatBytes, formatCount, renewalPhrase, statusClass, statusLabel, type Page } from "../store.svelte";

  let { onGo }: { onGo: (page: Page) => void } = $props();

  let traffic = $derived(data.traffic);
  let overview = $derived(data.overview);

  let maxRequests = $derived(
    traffic?.available ? Math.max(1, ...traffic.hourly.map((b) => b.requests)) : 1,
  );

  let errorRate = $derived(
    overview && overview.requests_24h > 0
      ? Math.round((overview.errors_24h / overview.requests_24h) * 1000) / 10
      : 0,
  );

  function hourLabel(iso: string): string {
    return new Date(iso).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  }

  function timeLabel(iso: string): string {
    return new Date(iso).toLocaleTimeString(undefined, {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  }

  let recent = $derived(traffic?.recent.slice(0, 8) ?? []);
</script>

<div style="display:flex; flex-direction:column; gap: var(--space-6)">
  <div class="grid-stats">
    <div class="panel">
      <div class="stat-label">Hosts live</div>
      <div class="stat-value">{data.liveCount}</div>
      <div class="stat-foot">of {data.hostCount} configured</div>
    </div>

    <div class="panel">
      <div class="stat-label">Certificates</div>
      {#if overview?.certs_available}
        <div class="stat-value">{overview.cert_count}</div>
        <div class="stat-foot">
          {#if overview.earliest_expiring_domain && overview.earliest_expiring_at}
            next renews {renewalPhrase(overview.earliest_expiring_at)}
          {:else}
            none issued yet
          {/if}
        </div>
      {:else}
        <div class="stat-value" style="font-size:22px; color: var(--color-neutral-600)">—</div>
        <div class="stat-foot">storage not readable</div>
      {/if}
    </div>

    <div class="panel">
      <div class="stat-label">Requests · 24h</div>
      {#if overview?.traffic_available}
        <div class="stat-value">{formatCount(overview.requests_24h)}</div>
        <div class="stat-foot">{errorRate}% errors</div>
      {:else}
        <div class="stat-value" style="font-size:22px; color: var(--color-neutral-600)">—</div>
        <div class="stat-foot">access log not configured</div>
      {/if}
    </div>

    <div class="panel stat-sage">
      <div class="stat-label">Footprint</div>
      <div class="stat-value">{overview ? formatBytes(overview.footprint_bytes) : "—"}</div>
      <div class="stat-foot">caddy-ui, resident</div>
    </div>
  </div>

  <div class="grid-halves">
    <div class="panel" style="display:flex; flex-direction:column; gap: var(--space-4)">
      <div style="display:flex; align-items:baseline; gap: var(--space-3)">
        <div class="panel-title">Traffic</div>
        <div style="font-size:13px; color: var(--color-neutral-600); margin-left:auto">last 24 hours</div>
      </div>

      {#if !traffic?.available}
        <div class="text-muted" style="font-size:13px">
          Access logging isn't configured for this instance.
        </div>
      {:else if traffic.total_24h === 0}
        <div class="text-muted" style="font-size:13px">No requests logged in the last 24 hours.</div>
      {:else}
        <div class="chart">
          {#each traffic.hourly as bucket (bucket.hour)}
            <div
              class="chart-col"
              title={`${hourLabel(bucket.hour)} — ${bucket.requests} request${bucket.requests === 1 ? "" : "s"}, ${bucket.errors} error${bucket.errors === 1 ? "" : "s"}`}
            >
              {#if bucket.errors > 0}
                <div class="chart-err" style={`height:${(bucket.errors / maxRequests) * 100}%`}></div>
              {/if}
              {#if bucket.requests - bucket.errors > 0}
                <div
                  class="chart-ok"
                  style={`height:${((bucket.requests - bucket.errors) / maxRequests) * 100}%`}
                ></div>
              {/if}
            </div>
          {/each}
        </div>
        <div class="chart-axis">
          <span>{hourLabel(traffic.hourly[0].hour)}</span>
          <span>{hourLabel(traffic.hourly[12].hour)}</span>
          <span>now</span>
        </div>
      {/if}
    </div>

    <div class="panel" style="display:flex; flex-direction:column; gap: var(--space-3)">
      <div style="display:flex; align-items:baseline; gap: var(--space-3)">
        <div class="panel-title">Recent requests</div>
        {#if recent.length > 0}
          <button class="btn btn-ghost" onclick={() => onGo("logs")} style="margin-left:auto; font-size:13px">
            All logs
          </button>
        {/if}
      </div>

      {#if recent.length === 0}
        <div class="text-muted" style="font-size:13px">
          Nothing logged yet. Requests appear here as Caddy handles them.
        </div>
      {:else}
        {#each recent as entry, i (i)}
          <div
            style="display:flex; gap: var(--space-3); align-items:baseline; padding: var(--space-2) 0; border-top: 1px solid var(--color-divider)"
          >
            <span class={statusClass(entry.status)} style="font-family: var(--font-mono); font-size:13px; flex:none">
              {statusLabel(entry.status)}
            </span>
            <div style="min-width:0; flex:1">
              <div style="font-size:14px; font-weight:600; overflow:hidden; text-overflow:ellipsis; white-space:nowrap">
                {entry.domain}
              </div>
              <div style="font-size:13px; color: var(--color-neutral-600)">
                {timeLabel(entry.time)} · {Math.round(entry.duration * 1000)}ms
              </div>
            </div>
          </div>
        {/each}
      {/if}
    </div>
  </div>
</div>
