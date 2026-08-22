<script lang="ts">
  import type { StatusResponse } from "../types";

  type Page = "hosts" | "import-export";

  let { page = $bindable<Page>(), status }: { page: Page; status: StatusResponse } = $props();

  const items: { key: Page | "certificates" | "access" | "logs"; label: string; enabled: boolean }[] = [
    { key: "hosts", label: "Proxy Hosts", enabled: true },
    { key: "import-export", label: "Import / Export", enabled: true },
    { key: "certificates", label: "Certificates", enabled: false },
    { key: "access", label: "Access", enabled: false },
    { key: "logs", label: "Logs", enabled: false },
  ];
</script>

<nav class="sidebar">
  <div class="brand">Caddy Proxy UI</div>
  {#each items as item (item.key)}
    <button
      class:active={page === item.key}
      class:disabled={!item.enabled}
      disabled={!item.enabled}
      onclick={() => item.enabled && (page = item.key as Page)}
    >
      {item.label}{!item.enabled ? " · soon" : ""}
    </button>
  {/each}
  <div style="flex:1"></div>
  <div class="muted" style="padding: 12px 20px 0;">{status.host_count} host{status.host_count === 1 ? "" : "s"}</div>
  {#if status.version}
    <div class="muted" style="padding: 2px 20px 0;">v{status.version}</div>
  {/if}
</nav>
