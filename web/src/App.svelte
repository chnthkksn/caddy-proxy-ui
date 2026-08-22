<script lang="ts">
  import { onMount } from "svelte";
  import { api } from "./api";
  import type { StatusResponse } from "./types";
  import Setup from "./lib/Setup.svelte";
  import Login from "./lib/Login.svelte";
  import Header from "./lib/Header.svelte";
  import Sidebar from "./lib/Sidebar.svelte";
  import HostsPage from "./lib/HostsPage.svelte";
  import ImportExportPage from "./lib/ImportExportPage.svelte";

  type View = "loading" | "setup" | "login" | "app";
  type Page = "hosts" | "import-export";

  let view = $state<View>("loading");
  let page = $state<Page>("hosts");
  let status = $state<StatusResponse>({
    setup_required: false,
    caddy_connected: false,
    host_count: 0,
    version: "",
  });

  async function refreshStatus() {
    try {
      status = await api.status();
    } catch {
      // transient — keep the last known status rather than flapping the UI
    }
  }

  async function init() {
    try {
      status = await api.status();
      if (status.setup_required) {
        view = "setup";
        return;
      }
      await api.listHosts();
      view = "app";
    } catch {
      view = "login";
    }
  }

  onMount(() => {
    init();
    const interval = setInterval(refreshStatus, 15000);
    return () => clearInterval(interval);
  });

  function onSetupDone() {
    view = "login";
  }

  async function onLoggedIn() {
    await refreshStatus();
    view = "app";
  }

  async function onLogout() {
    await api.logout();
    view = "login";
  }
</script>

{#if view === "loading"}
  <div class="centered">Loading…</div>
{:else if view === "setup"}
  <Setup onDone={onSetupDone} />
{:else if view === "login"}
  <Login onLoggedIn={onLoggedIn} />
{:else if view === "app"}
  <div class="shell">
    <Sidebar bind:page {status} />
    <div class="main">
      <Header {status} onLogout={onLogout} />
      {#if page === "hosts"}
        <HostsPage onChange={refreshStatus} />
      {:else if page === "import-export"}
        <ImportExportPage onChange={refreshStatus} />
      {/if}
    </div>
  </div>
{/if}
