<script lang="ts">
  import { onMount } from "svelte";
  import { api } from "./api";
  import { data, theme, type Page } from "./store.svelte";
  import type { Host } from "./types";

  import Icon from "./lib/Icon.svelte";
  import Setup from "./lib/Setup.svelte";
  import Login from "./lib/Login.svelte";
  import Rail from "./lib/Rail.svelte";
  import CaddyfileDrawer from "./lib/CaddyfileDrawer.svelte";

  import OverviewPage from "./lib/OverviewPage.svelte";
  import HostsPage from "./lib/HostsPage.svelte";
  import HostEditPage from "./lib/HostEditPage.svelte";
  import CertificatesPage from "./lib/CertificatesPage.svelte";
  import AccessPage from "./lib/AccessPage.svelte";
  import LogsPage from "./lib/LogsPage.svelte";
  import CaddyfilePage from "./lib/CaddyfilePage.svelte";
  import SettingsPage from "./lib/SettingsPage.svelte";

  type View = "loading" | "setup" | "login" | "app";

  let view = $state<View>("loading");
  let page = $state<Page>("overview");
  let navOpen = $state(false);
  let drawerOpen = $state(false);
  let editing = $state<Host | null>(null); // null while on "edit" = creating

  // Blurbs are the design's own copy — they explain what each page is for
  // rather than restating its title.
  const titles: Record<Page, [string, string]> = {
    overview: ["Overview", "One binary, one database, one page that tells you everything is fine."],
    hosts: ["Proxy hosts", "A domain and an upstream. Caddy does the rest, live, with no restart."],
    edit: ["Host", "Two fields are required. Everything else has a sensible default."],
    certificates: ["Certificates", "Issued and renewed by Caddy. This page is a window, not a manager."],
    access: ["Access lists", "IP allow lists and basic auth, attached to hosts."],
    logs: ["Logs & traffic", "The access log, tailed. Nothing is stored beyond Caddy's own rotation."],
    caddyfile: ["Caddyfile", "What your hosts become. Import is conservative; export is yours to keep."],
    settings: ["Settings", "Where this instance points, and who can sign in."],
  };

  let title = $derived(titles[page][0]);
  let blurb = $derived(titles[page][1]);

  async function init() {
    try {
      const status = await api.status();
      data.status = status;
      if (status.setup_required) {
        view = "setup";
        return;
      }
      // Show the shell as soon as we know the session is valid. The rest of
      // the dashboard data fills in behind it, so a slow or unreachable Caddy
      // never leaves the whole app stuck on a loading screen — which is
      // exactly the moment someone needs to get in and look.
      await data.loadHosts();
      view = "app";
      data.loadAll();
    } catch {
      view = "login";
    }
  }

  onMount(() => {
    init();
    const interval = setInterval(() => {
      if (view === "app") data.refreshStatus();
    }, 15000);
    return () => clearInterval(interval);
  });

  async function onLoggedIn() {
    await data.loadAll();
    view = "app";
  }

  async function onLogout() {
    await api.logout();
    data.clear();
    page = "overview";
    view = "login";
  }

  function openCreate() {
    editing = null;
    page = "edit";
  }

  function openEdit(host: Host) {
    editing = host;
    page = "edit";
  }

  async function retrySync() {
    const sync = await api.sync();
    data.noteSync(sync);
    await data.refreshStatus();
  }
</script>

<svelte:head>
  <meta name="color-scheme" content={theme.value} />
</svelte:head>

{#if view === "loading"}
  <div class="auth" data-theme={theme.value}>
    <div class="text-muted">Loading…</div>
  </div>
{:else if view === "setup"}
  <Setup onDone={() => (view = "login")} />
{:else if view === "login"}
  <Login {onLoggedIn} />
{:else}
  <div class="app" data-theme={theme.value}>
    <div class="shell">
      <div class="mobilebar">
        <button class="icon-btn" onclick={() => (navOpen = true)} aria-label="Open menu" style="width:40px;height:40px">
          <Icon name="menu" size={20} />
        </button>
        <div style="display:flex; align-items:center; gap: var(--space-2); min-width:0">
          <div class="brand-mark" style="width:28px;height:28px"><Icon name="shield" size={15} /></div>
          <div class="brand-name" style="font-size:16px">caddy-ui</div>
        </div>
        <span class="status-line" style="margin-left:auto; font-size:12px; color: var(--color-neutral-700)">
          <span class="dot" class:dot-on={data.status.caddy_connected} class:dot-off={!data.status.caddy_connected}></span>
          {data.status.caddy_connected ? "Live" : "Offline"}
        </span>
        <button class="icon-btn" onclick={() => theme.toggle()} aria-label="Switch theme" style="width:40px;height:40px">
          <Icon name="theme" size={18} />
        </button>
      </div>

      {#if navOpen}
        <div
          class="scrim-nav"
          role="presentation"
          onclick={() => (navOpen = false)}
          onkeydown={(e) => e.key === "Escape" && (navOpen = false)}
        ></div>
      {/if}

      <Rail bind:page open={navOpen} onNavigate={() => (navOpen = false)} {onLogout} />

      <main class="main">
        <header class="pagehead">
          <div style="min-width:0">
            <h1>{title}</h1>
            <div class="blurb">{blurb}</div>
          </div>
          <div class="actions">
            {#if page === "hosts" || page === "overview"}
              <button class="btn btn-secondary" onclick={() => (drawerOpen = true)}>
                <Icon name="caddyfile" size={16} />
                View Caddyfile
              </button>
              <button class="btn btn-primary" onclick={openCreate}>Add host</button>
            {/if}
          </div>
        </header>

        {#if !data.status.caddy_connected}
          <div class="notice">
            <Icon name="warn" size={18} />
            <span>
              <strong>Caddy is unreachable.</strong> Changes are saved to SQLite and will be pushed
              as soon as the admin API answers again.
            </span>
            <button class="btn btn-ghost" onclick={retrySync} style="margin-left:auto; font-size:13px">
              Retry now
            </button>
          </div>
        {:else if data.syncWarning}
          <div class="notice">
            <Icon name="warn" size={18} />
            <span><strong>Last change didn't reach Caddy.</strong> {data.syncWarning}</span>
            <button class="btn btn-ghost" onclick={retrySync} style="margin-left:auto; font-size:13px">
              Retry now
            </button>
          </div>
        {/if}

        {#if data.error}
          <div class="notice notice-danger">
            <Icon name="warn" size={18} />
            <span>{data.error}</span>
          </div>
        {/if}

        <div class="page">
          {#if page === "overview"}
            <OverviewPage onGo={(p) => (page = p)} />
          {:else if page === "hosts"}
            <HostsPage onEdit={openEdit} onCreate={openCreate} />
          {:else if page === "edit"}
            <!-- Keyed so switching straight from one host's editor to another
                 remounts the form instead of keeping the first host's values. -->
            {#key editing?.id ?? "new"}
              <HostEditPage host={editing} onDone={() => (page = "hosts")} />
            {/key}
          {:else if page === "certificates"}
            <CertificatesPage />
          {:else if page === "access"}
            <AccessPage />
          {:else if page === "logs"}
            <LogsPage />
          {:else if page === "caddyfile"}
            <CaddyfilePage />
          {:else if page === "settings"}
            <SettingsPage {onLogout} />
          {/if}
        </div>
      </main>
    </div>

    {#if drawerOpen}
      <CaddyfileDrawer onClose={() => (drawerOpen = false)} />
    {/if}
  </div>
{/if}
