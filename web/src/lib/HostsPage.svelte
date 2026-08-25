<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";
  import { data, formatCount, renewalPhrase } from "../store.svelte";
  import type { Host } from "../types";
  import Icon from "./Icon.svelte";
  import ConfirmDialog from "./ConfirmDialog.svelte";

  let { onEdit, onCreate }: { onEdit: (host: Host) => void; onCreate: () => void } = $props();

  let query = $state("");
  let viewMode = $state<"table" | "cards">("table");
  let statusFilter = $state<"all" | "live" | "paused">("all");
  let error = $state("");
  let pending = $state<{ host: Host; kind: "toggle" | "delete" } | null>(null);

  let visible = $derived(
    data.hosts.filter((h) => {
      if (statusFilter === "live" && !h.enabled) return false;
      if (statusFilter === "paused" && h.enabled) return false;
      const q = query.trim().toLowerCase();
      if (!q) return true;
      return (
        h.domain.toLowerCase().includes(q) ||
        h.upstream.toLowerCase().includes(q) ||
        h.group_label.toLowerCase().includes(q)
      );
    }),
  );

  function tlsLabel(host: Host): string {
    const cert = data.certFor(host.domain);
    if (cert) return `renews ${renewalPhrase(cert.not_after)}`;
    if (!host.enabled) return "—";
    if (data.certs && !data.certs.available) return "unknown";
    return "pending";
  }

  function reqLabel(host: Host): string {
    const n = data.reqsFor(host.domain);
    return n === null ? "—" : formatCount(n);
  }

  async function runPending() {
    if (!pending) return;
    const { host, kind } = pending;
    pending = null;
    error = "";
    try {
      const result = kind === "delete" ? await api.deleteHost(host.id) : await api.toggleHost(host.id);
      data.noteSync(result.sync);
      await data.loadAll();
    } catch (err) {
      error = errorMessage(err);
    }
  }
</script>

<div style="display:flex; flex-direction:column; gap: var(--space-4)">
  {#if error}
    <div class="notice notice-danger"><Icon name="warn" size={18} /><span>{error}</span></div>
  {/if}

  <div style="display:flex; gap: var(--space-3); flex-wrap:wrap; align-items:center">
    <input
      class="input"
      type="search"
      placeholder="Search domain or upstream…"
      bind:value={query}
      style="flex: 1 1 240px; min-width:0"
    />
    <div class="seg" style="flex:none">
      <button class="seg-opt" class:is-on={viewMode === "table"} onclick={() => (viewMode = "table")}>Table</button>
      <button class="seg-opt" class:is-on={viewMode === "cards"} onclick={() => (viewMode = "cards")}>Cards</button>
    </div>
    <div class="seg" style="flex:none">
      <button class="seg-opt" class:is-on={statusFilter === "all"} onclick={() => (statusFilter = "all")}>All</button>
      <button class="seg-opt" class:is-on={statusFilter === "live"} onclick={() => (statusFilter = "live")}>Live</button>
      <button class="seg-opt" class:is-on={statusFilter === "paused"} onclick={() => (statusFilter = "paused")}>Paused</button>
    </div>
  </div>

  {#if viewMode === "table"}
    <div class="panel" style="padding: var(--space-3) var(--space-6) var(--space-4); overflow-x:auto">
      <table class="table" style="min-width:720px">
        <thead>
          <tr>
            <th>Domain</th>
            <th>Upstream</th>
            <th>TLS</th>
            <th style="text-align:right">Req / 24h</th>
            <th>Status</th>
            <th style="text-align:right">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each visible as host (host.id)}
            <tr>
              <td>
                {#if host.group_label}<div class="kicker">{host.group_label}</div>{/if}
                <a href={`https://${host.domain}`} target="_blank" rel="noopener noreferrer" style="font-weight:700; font-size:15px">
                  {host.domain} ↗
                </a>
              </td>
              <td class="mono" style="font-size:13px; color: var(--color-neutral-700)">{host.upstream}</td>
              <td style="font-size:13px; color: var(--color-neutral-700); white-space:nowrap">{tlsLabel(host)}</td>
              <td class="mono" style="text-align:right; font-size:13px; color: var(--color-neutral-700)">{reqLabel(host)}</td>
              <td>
                <span class="tag" class:tag-accent-2={host.enabled} class:tag-neutral={!host.enabled}>
                  {host.enabled ? "live" : "paused"}
                </span>
              </td>
              <td>
                <div style="display:flex; gap:6px; justify-content:flex-end; align-items:center">
                  <button class="btn btn-secondary" onclick={() => onEdit(host)} style="font-size:12px; padding:6px 14px">Edit</button>
                  <button class="btn btn-ghost" onclick={() => (pending = { host, kind: "toggle" })} style="font-size:12px; padding:6px 12px">
                    {host.enabled ? "Pause" : "Resume"}
                  </button>
                  <button
                    class="btn btn-ghost btn-danger"
                    onclick={() => (pending = { host, kind: "delete" })}
                    aria-label={`Delete ${host.domain}`}
                    title="Delete host"
                    style="font-size:12px; padding:6px 10px"
                  >
                    <Icon name="trash" size={15} />
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>

      {#if visible.length === 0}
        <div style="padding: var(--space-6) 0; color: var(--color-neutral-600); font-size:14px">
          {data.hosts.length === 0 ? "No proxy hosts yet." : "No hosts match this filter."}
        </div>
      {/if}
    </div>
  {:else}
    <div class="grid-cards">
      {#each visible as host (host.id)}
        <div class="panel" style="display:flex; flex-direction:column; gap: var(--space-4)">
          <div style="display:flex; align-items:flex-start; gap: var(--space-3)">
            <div style="min-width:0; flex:1">
              {#if host.group_label}<div class="kicker" style="font-size:12px">{host.group_label}</div>{/if}
              <div style="font-size:19px; font-weight:700; margin-top:4px; overflow-wrap:anywhere">{host.domain}</div>
            </div>
            <span class="tag" class:tag-accent-2={host.enabled} class:tag-neutral={!host.enabled}>
              {host.enabled ? "live" : "paused"}
            </span>
          </div>

          <div class="mono" style="display:flex; align-items:center; gap: var(--space-2); font-size:13px; color: var(--color-neutral-700)">
            <Icon name="arrow" size={15} />
            {host.upstream}
          </div>

          <div style="display:flex; align-items:center; gap: var(--space-3); font-size:13px; color: var(--color-neutral-700); flex-wrap:wrap">
            <span style="display:inline-flex; align-items:center; gap:6px">
              <Icon name="certificates" size={14} />
              TLS {tlsLabel(host)}
            </span>
            <span>{reqLabel(host)} req/24h</span>
          </div>

          <div style="display:flex; gap: var(--space-2); align-items:center; flex-wrap:wrap; border-top:1px solid var(--color-divider); padding-top: var(--space-3)">
            <button class="btn btn-secondary" onclick={() => onEdit(host)} style="font-size:13px">Edit</button>
            <button class="btn btn-ghost" onclick={() => (pending = { host, kind: "toggle" })} style="font-size:13px">
              {host.enabled ? "Pause" : "Resume"}
            </button>
            <button
              class="btn btn-ghost btn-danger"
              onclick={() => (pending = { host, kind: "delete" })}
              aria-label={`Delete ${host.domain}`}
              title="Delete host"
              style="font-size:13px; padding-inline:12px"
            >
              <Icon name="trash" size={15} />
            </button>
            <a href={`https://${host.domain}`} target="_blank" rel="noopener noreferrer" style="margin-left:auto; font-size:13px; font-weight:600">
              Open ↗
            </a>
          </div>
        </div>
      {/each}

      <button class="tile-new" onclick={onCreate}>
        <Icon name="plus" size={26} />
        New proxy host
        <span class="sub">domain → upstream, that's it</span>
      </button>
    </div>
  {/if}
</div>

{#if pending}
  <ConfirmDialog
    title={pending.kind === "delete" ? `Delete ${pending.host.domain}?` : pending.host.enabled ? `Pause ${pending.host.domain}?` : `Resume ${pending.host.domain}?`}
    body={pending.kind === "delete"
      ? "The host is removed from SQLite and dropped from Caddy's config on the next push. Its certificate stays in Caddy's storage until it expires."
      : pending.host.enabled
        ? "The host stays saved here but leaves Caddy's config, so it stops answering immediately."
        : "The host goes back into Caddy's config and starts answering again."}
    confirmLabel={pending.kind === "delete" ? "Delete host" : pending.host.enabled ? "Pause host" : "Resume host"}
    danger={pending.kind === "delete"}
    onConfirm={runPending}
    onCancel={() => (pending = null)}
  />
{/if}
