<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";
  import { data, statusClass, statusLabel } from "../store.svelte";
  import Icon from "./Icon.svelte";

  let hostFilter = $state("all");
  let statusFilter = $state<"all" | "4xx" | "5xx">("all");
  let refreshing = $state(false);
  let error = $state("");

  let traffic = $derived(data.traffic);

  let lines = $derived(
    (traffic?.recent ?? []).filter((entry) => {
      if (hostFilter !== "all" && entry.domain !== hostFilter) return false;
      if (statusFilter === "4xx" && (entry.status < 400 || entry.status >= 500)) return false;
      if (statusFilter === "5xx" && entry.status < 500) return false;
      return true;
    }),
  );

  // Only offer hosts that actually appear in the log — filtering by a host
  // with no traffic would just look broken.
  let hostOptions = $derived([
    ...new Set([...(traffic?.recent ?? []).map((e) => e.domain)].filter(Boolean)),
  ].sort());

  function timeLabel(iso: string): string {
    return new Date(iso).toLocaleTimeString(undefined, {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    });
  }

  async function refresh() {
    refreshing = true;
    error = "";
    try {
      data.traffic = await api.traffic();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      refreshing = false;
    }
  }
</script>

<div style="display:flex; flex-direction:column; gap: var(--space-4)">
  {#if error}
    <div class="notice notice-danger"><Icon name="warn" size={18} /><span>{error}</span></div>
  {/if}

  {#if traffic && !traffic.available}
    <div class="notice">
      <Icon name="warn" size={18} />
      <span>
        <strong>Access logging isn't configured.</strong> Set ACCESS_LOG_PATH so Caddy writes its
        access log somewhere caddy-ui can read.
      </span>
    </div>
  {:else}
    <div style="display:flex; gap: var(--space-3); flex-wrap:wrap; align-items:center">
      <label class="field" style="flex-direction:row; align-items:center; gap: var(--space-2); flex: 0 1 320px; min-width:0">
        <span class="field-label" style="flex:none">Host</span>
        <select class="input" bind:value={hostFilter} style="flex:1; min-width:0">
          <option value="all">All hosts</option>
          {#each hostOptions as domain (domain)}
            <option value={domain}>{domain}</option>
          {/each}
        </select>
      </label>

      <div class="seg">
        <button class="seg-opt" class:is-on={statusFilter === "all"} onclick={() => (statusFilter = "all")}>All</button>
        <button class="seg-opt" class:is-on={statusFilter === "4xx"} onclick={() => (statusFilter = "4xx")}>4xx</button>
        <button class="seg-opt" class:is-on={statusFilter === "5xx"} onclick={() => (statusFilter = "5xx")}>5xx</button>
      </div>

      <button class="btn btn-secondary" onclick={refresh} disabled={refreshing} style="margin-left:auto; font-size:14px">
        <Icon name="refresh" size={16} />
        {refreshing ? "Refreshing…" : "Refresh"}
      </button>
    </div>

    <div class="ink">
      <div class="logrows">
        {#each lines as line, i (i)}
          <div class="logrow">
            <span class="at">{timeLabel(line.time)}</span>
            <span class={statusClass(line.status)}>{statusLabel(line.status)}</span>
            <span class="req">{line.domain}</span>
            <span class="ms">{Math.round(line.duration * 1000)}ms</span>
          </div>
        {/each}

        {#if lines.length === 0}
          <div style="color: var(--ink-dim); padding: var(--space-4) 0">
            No matching requests in the current window.
          </div>
        {/if}
      </div>
    </div>

    <div style="font-size:13px; color: var(--color-neutral-600)">
      Showing {lines.length}
      {lines.length === 1 ? "request" : "requests"}
      {hostFilter === "all" ? "across all hosts" : `for ${hostFilter}`} ·
      {traffic?.total_24h ?? 0} in the last 24h.
    </div>
  {/if}
</div>
