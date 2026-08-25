<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";
  import { data, theme } from "../store.svelte";
  import type { InstanceSettings } from "../types";
  import Icon from "./Icon.svelte";

  let { onLogout }: { onLogout: () => void } = $props();

  let settings = $state<InstanceSettings | null>(null);
  let loadError = $state("");

  let currentPassword = $state("");
  let newPassword = $state("");
  let confirmPassword = $state("");
  let pwError = $state("");
  let pwDone = $state(false);
  let saving = $state(false);
  let testing = $state(false);

  (async () => {
    try {
      settings = await api.settings();
    } catch (err) {
      loadError = errorMessage(err);
    }
  })();

  async function testConnection() {
    testing = true;
    try {
      const sync = await api.sync();
      data.noteSync(sync);
      await data.refreshStatus();
    } finally {
      testing = false;
    }
  }

  async function changePassword(e: SubmitEvent) {
    e.preventDefault();
    pwError = "";
    pwDone = false;
    saving = true;
    try {
      await api.changePassword(currentPassword, newPassword, confirmPassword);
      pwDone = true;
      currentPassword = newPassword = confirmPassword = "";
    } catch (err) {
      pwError = errorMessage(err);
    } finally {
      saving = false;
    }
  }
</script>

<div style="display:flex; flex-direction:column; gap: var(--space-4)">
  {#if loadError}
    <div class="notice notice-danger"><Icon name="warn" size={18} /><span>{loadError}</span></div>
  {/if}

  <div class="grid-forms">
    <div class="panel" style="display:flex; flex-direction:column; gap: var(--space-4)">
      <div class="panel-title">Connection</div>
      <div style="font-size:13px; color: var(--color-neutral-700); line-height:1.6; margin-top:-6px">
        Set by environment variable when the service starts, so they're shown here for
        troubleshooting rather than edited in the browser.
      </div>

      <div class="field">
        <span class="field-label">Caddy admin API</span>
        <input class="input mono" value={settings?.caddy_admin_url ?? "…"} readonly />
      </div>
      <div class="field">
        <span class="field-label">Access log</span>
        <input class="input mono" value={settings?.access_log_path || "not configured"} readonly />
      </div>
      <div class="field">
        <span class="field-label">Caddy certificate storage</span>
        <input class="input mono" value={settings?.cert_storage_path || "not configured"} readonly />
      </div>

      <div style="display:flex; align-items:center; gap: var(--space-3); flex-wrap:wrap">
        <button class="btn btn-secondary" onclick={testConnection} disabled={testing}>
          {testing ? "Testing…" : "Test connection"}
        </button>
        <span class="status-line" style="font-size:13px; color: var(--color-neutral-700)">
          <span class="dot" class:dot-on={data.status.caddy_connected} class:dot-off={!data.status.caddy_connected}></span>
          {data.status.caddy_connected ? "Caddy connected" : "Caddy offline"}
        </span>
      </div>
    </div>

    <div style="display:flex; flex-direction:column; gap: var(--space-4)">
      <form class="panel" onsubmit={changePassword} style="display:flex; flex-direction:column; gap: var(--space-4)">
        <div class="panel-title">Administrator</div>
        <div style="font-size:13px; color: var(--color-neutral-700); line-height:1.6; margin-top:-6px">
          Single account, no roles. If you're locked out, reset it on the server with
          <span class="mono">caddy-ui reset-password</span>.
        </div>

        <label class="field">
          <span class="field-label">Current password</span>
          <input class="input" type="password" bind:value={currentPassword} required />
        </label>
        <label class="field">
          <span class="field-label">New password</span>
          <input class="input" type="password" bind:value={newPassword} minlength="8" required />
          <span class="field-hint">At least 8 characters.</span>
        </label>
        <label class="field">
          <span class="field-label">Confirm new password</span>
          <input class="input" type="password" bind:value={confirmPassword} minlength="8" required />
        </label>

        {#if pwError}
          <div class="notice notice-danger" style="margin:0"><Icon name="warn" size={18} /><span>{pwError}</span></div>
        {/if}
        {#if pwDone}
          <div class="notice" style="margin:0; background: var(--color-accent-2-200); color: var(--color-accent-2-900)">
            <Icon name="warn" size={18} />
            <span>Password changed.</span>
          </div>
        {/if}

        <div style="display:flex; gap: var(--space-2); align-items:center">
          <button class="btn btn-primary" type="submit" disabled={saving}>
            {saving ? "Saving…" : "Change password"}
          </button>
          <button class="btn btn-ghost" type="button" onclick={onLogout} style="margin-left:auto">Log out</button>
        </div>
      </form>

      <div class="panel" style="display:flex; flex-direction:column; gap: var(--space-3)">
        <div class="panel-title">Appearance</div>
        <div style="display:flex; align-items:center; justify-content:space-between; gap: var(--space-3)">
          <div style="font-size:14px; color: var(--color-neutral-700)">
            {theme.value === "dark" ? "Dark" : "Light"} theme
          </div>
          <div class="seg">
            <button class="seg-opt" class:is-on={theme.value === "light"} onclick={() => theme.value === "dark" && theme.toggle()}>
              Light
            </button>
            <button class="seg-opt" class:is-on={theme.value === "dark"} onclick={() => theme.value === "light" && theme.toggle()}>
              Dark
            </button>
          </div>
        </div>
        <div class="mono" style="border-top:1px solid var(--color-divider); padding-top: var(--space-3); font-size:13px; color: var(--color-neutral-600)">
          caddy-ui {settings?.version ?? data.status.version ?? "dev"}
        </div>
      </div>
    </div>
  </div>
</div>
