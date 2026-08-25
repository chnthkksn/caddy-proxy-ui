<script lang="ts">
  import Icon from "./Icon.svelte";
  import { data, theme, type Page } from "../store.svelte";

  let {
    page = $bindable<Page>(),
    open = false,
    onNavigate,
    onLogout,
  }: {
    page: Page;
    open?: boolean;
    onNavigate: () => void;
    onLogout: () => void;
  } = $props();

  const items: { key: Page; label: string; icon: string; badge?: boolean }[] = [
    { key: "overview", label: "Overview", icon: "overview" },
    { key: "hosts", label: "Proxy hosts", icon: "hosts", badge: true },
    { key: "certificates", label: "Certificates", icon: "certificates" },
    { key: "access", label: "Access lists", icon: "access" },
    { key: "logs", label: "Logs & traffic", icon: "logs" },
    { key: "caddyfile", label: "Caddyfile", icon: "caddyfile" },
    { key: "settings", label: "Settings", icon: "settings" },
  ];

  function go(key: Page) {
    page = key;
    onNavigate();
  }

  // The hosts page and the host editor are the same destination as far as
  // the rail is concerned — editing shouldn't blank out the active marker.
  let activeKey = $derived(page === "edit" ? "hosts" : page);

  let footprint = $derived(
    data.overview ? `${Math.round(data.overview.footprint_bytes / 1024 / 1024)} MB resident` : "",
  );
</script>

<aside class="rail" class:is-open={open}>
  <div class="brand">
    <div class="brand-mark"><Icon name="shield" size={18} /></div>
    <div class="brand-name">caddy-ui</div>
  </div>

  <nav class="nav">
    {#each items as item (item.key)}
      <button class:is-active={activeKey === item.key} onclick={() => go(item.key)}>
        <Icon name={item.icon} size={18} />
        {item.label}
        {#if item.badge}<span class="count">{data.hostCount}</span>{/if}
      </button>
    {/each}
  </nav>

  <div class="rail-foot">
    <div class="status-card">
      <div class="status-line">
        <span class="dot" class:dot-on={data.status.caddy_connected} class:dot-off={!data.status.caddy_connected}></span>
        {data.status.caddy_connected ? "Caddy connected" : "Caddy offline"}
      </div>
      {#if footprint}
        <div class="status-meta">{footprint}</div>
      {/if}
    </div>

    <div style="display:flex; align-items:center; gap: var(--space-2)">
      <button class="btn btn-ghost" onclick={onLogout} style="justify-content:flex-start; flex:1">
        Log out
      </button>
      <a
        class="icon-btn"
        href="https://github.com/chnthkksn/caddy-proxy-ui"
        target="_blank"
        rel="noreferrer"
        aria-label="Open the GitHub repository"
        title="GitHub repository"
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 2a10 10 0 0 0-3.16 19.49c.5.09.68-.22.68-.48v-1.7c-2.78.6-3.37-1.34-3.37-1.34-.45-1.16-1.11-1.47-1.11-1.47-.91-.62.07-.61.07-.61 1 .07 1.53 1.03 1.53 1.03.89 1.53 2.34 1.09 2.91.83.09-.65.35-1.09.63-1.34-2.22-.25-4.56-1.11-4.56-4.95 0-1.09.39-1.98 1.03-2.68-.1-.25-.45-1.27.1-2.65 0 0 .84-.27 2.75 1.02a9.5 9.5 0 0 1 5 0c1.91-1.29 2.75-1.02 2.75-1.02.55 1.38.2 2.4.1 2.65.64.7 1.03 1.59 1.03 2.68 0 3.85-2.35 4.7-4.58 4.94.36.31.68.92.68 1.86v2.75c0 .27.18.58.69.48A10 10 0 0 0 12 2Z" /></svg>
      </a>
      <button
        class="icon-btn"
        onclick={() => theme.toggle()}
        aria-label="Switch theme"
        title={theme.value === "dark" ? "Switch to light" : "Switch to dark"}
      >
        <Icon name="theme" size={18} />
      </button>
    </div>
  </div>
</aside>
