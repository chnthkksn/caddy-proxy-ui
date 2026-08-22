<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";
  import type { Host, HostMutationResult, SyncStatus } from "../types";
  import HostModal from "./HostModal.svelte";

  let { onChange }: { onChange?: () => void } = $props();

  let hosts = $state<Host[]>([]);
  let loading = $state(true);
  let error = $state("");
  let syncWarning = $state("");
  let modalOpen = $state(false);
  let editingHost = $state<Host | null>(null); // null = create

  async function load() {
    loading = true;
    error = "";
    try {
      hosts = await api.listHosts();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      loading = false;
    }
  }

  function noteSync(sync: SyncStatus) {
    syncWarning = !sync.synced
      ? `Saved, but Caddy is offline — it will sync automatically once reachable.${sync.error ? " (" + sync.error + ")" : ""}`
      : "";
    onChange?.();
  }

  function openCreate() {
    editingHost = null;
    modalOpen = true;
  }

  function openEdit(host: Host) {
    editingHost = host;
    modalOpen = true;
  }

  async function onSaved(result: HostMutationResult) {
    modalOpen = false;
    noteSync(result.sync);
    await load();
  }

  async function toggle(host: Host) {
    error = "";
    try {
      const result = await api.toggleHost(host.id);
      noteSync(result.sync);
      await load();
    } catch (err) {
      error = errorMessage(err);
    }
  }

  async function remove(host: Host) {
    if (!confirm(`Delete ${host.domain}?`)) return;
    error = "";
    try {
      const result = await api.deleteHost(host.id);
      noteSync(result.sync);
      await load();
    } catch (err) {
      error = errorMessage(err);
    }
  }

  load();
</script>

<div class="content">
  <div class="header-row">
    <h2>Proxy Hosts</h2>
    <button class="primary" onclick={openCreate}>+ Add Proxy</button>
  </div>

  {#if syncWarning}<div class="banner warn">{syncWarning}</div>{/if}
  {#if error}<div class="banner error">{error}</div>{/if}

  <div class="card">
    {#if loading}
      <div class="muted" style="padding:16px;">Loading…</div>
    {:else if hosts.length === 0}
      <div class="muted" style="padding:16px;">No proxy hosts yet.</div>
    {:else}
      <table>
        <thead>
          <tr>
            <th>Domain</th>
            <th>Upstream</th>
            <th>Status</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each hosts as host (host.id)}
            <tr>
              <td>{host.domain}</td>
              <td>{host.upstream}</td>
              <td>
                <button class="secondary" onclick={() => toggle(host)}>
                  {host.enabled ? "Enabled" : "Disabled"}
                </button>
              </td>
              <td style="text-align:right;">
                <button class="secondary" onclick={() => openEdit(host)}>Edit</button>
                <button class="secondary danger" onclick={() => remove(host)}>Delete</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>

{#if modalOpen}
  <HostModal host={editingHost} onClose={() => (modalOpen = false)} onSaved={onSaved} />
{/if}
