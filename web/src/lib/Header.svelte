<script lang="ts">
  import type { StatusResponse } from "../types";

  let { status, onLogout }: { status: StatusResponse; onLogout: () => void } = $props();
</script>

<header class="header">
  <div class="status-dot">
    <span class="dot" class:on={status.caddy_connected} class:off={!status.caddy_connected}></span>
    Caddy {status.caddy_connected ? "connected" : "offline"}
  </div>
  <button class="secondary" onclick={onLogout}>Log out</button>
</header>
