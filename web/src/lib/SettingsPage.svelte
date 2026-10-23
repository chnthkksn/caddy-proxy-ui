<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";
  import { data, theme } from "../store.svelte";
  import type { InstanceSettings, TOTPSetup } from "../types";
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

  // Two-factor: `setup` is non-null while a new secret is waiting to be
  // confirmed with a first code.
  let setup = $state<TOTPSetup | null>(null);
  let totpCode = $state("");
  let totpPassword = $state("");
  let totpError = $state("");
  let totpBusy = $state(false);
  let copied = $state(false);

  const groupedSecret = $derived(setup?.secret.match(/.{1,4}/g)?.join(" ") ?? "");

  async function totpAction(fn: () => Promise<void>) {
    totpError = "";
    totpBusy = true;
    try {
      await fn();
    } catch (err) {
      totpError = errorMessage(err);
    } finally {
      totpBusy = false;
    }
  }

  const startTOTPSetup = (e: SubmitEvent) => {
    e.preventDefault();
    return totpAction(async () => {
      setup = await api.totpSetup(totpPassword);
      totpPassword = totpCode = "";
      copied = false;
    });
  };

  function cancelTOTPSetup() {
    setup = null;
    totpCode = totpError = "";
  }

  async function copyURI() {
    if (!setup) return;
    try {
      await navigator.clipboard.writeText(setup.uri);
      copied = true;
    } catch {
      // Clipboard API needs a secure context; the link is still selectable.
    }
  }

  const enableTOTP = (e: SubmitEvent) => {
    e.preventDefault();
    return totpAction(async () => {
      await api.totpEnable(totpCode);
      setup = null;
      totpCode = "";
      if (settings) settings.totp_enabled = true;
    });
  };

  const disableTOTP = (e: SubmitEvent) => {
    e.preventDefault();
    return totpAction(async () => {
      await api.totpDisable(totpPassword, totpCode);
      totpPassword = totpCode = "";
      if (settings) settings.totp_enabled = false;
    });
  };

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

      <div class="panel" style="display:flex; flex-direction:column; gap: var(--space-4)">
        <div style="display:flex; align-items:center; gap: var(--space-2)">
          <div class="panel-title">Two-factor authentication</div>
          {#if settings}
            <span class="tag" class:tag-accent-2={settings.totp_enabled} class:tag-neutral={!settings.totp_enabled} style="margin-left:auto">
              {settings.totp_enabled ? "On" : "Off"}
            </span>
          {/if}
        </div>

        {#if settings?.totp_enabled}
          <form onsubmit={disableTOTP} style="display:flex; flex-direction:column; gap: var(--space-4)">
            <div style="font-size:13px; color: var(--color-neutral-700); line-height:1.6; margin-top:-6px">
              Sign-in asks for a code from your authenticator app after the password.
              Turning it off needs both. Lost the device? <span class="mono">caddy-ui reset-password</span>
              on the server turns it off too.
            </div>
            <label class="field">
              <span class="field-label">Password</span>
              <input class="input" type="password" bind:value={totpPassword} autocomplete="current-password" required />
            </label>
            <label class="field">
              <span class="field-label">Authentication code</span>
              <input class="input mono" bind:value={totpCode} inputmode="numeric" autocomplete="one-time-code" maxlength="7" required />
            </label>
            {#if totpError}
              <div class="notice notice-danger" style="margin:0"><Icon name="warn" size={18} /><span>{totpError}</span></div>
            {/if}
            <div>
              <button class="btn btn-secondary" type="submit" disabled={totpBusy}>
                {totpBusy ? "Turning off…" : "Turn off two-factor"}
              </button>
            </div>
          </form>
        {:else if setup}
          <form onsubmit={enableTOTP} style="display:flex; flex-direction:column; gap: var(--space-4)">
            <div style="font-size:13px; color: var(--color-neutral-700); line-height:1.6; margin-top:-6px">
              Scan this with your authenticator app (1Password, Bitwarden, Google Authenticator,
              Aegis…) or add the key by hand, then enter the code it shows to confirm.
            </div>
            {#if setup.qr}
              <!-- Always dark-on-white, whatever the theme: scanners need the contrast. -->
              <svg
                viewBox="0 0 {setup.qr.size} {setup.qr.size}"
                width="200"
                height="200"
                shape-rendering="crispEdges"
                role="img"
                aria-label="QR code for the two-factor setup link"
                style="align-self:center; border-radius: var(--radius-sm)"
              >
                <rect width={setup.qr.size} height={setup.qr.size} fill="#fff" />
                <path d={setup.qr.path} fill="#000" />
              </svg>
            {/if}
            <div class="field">
              <span class="field-label">Setup key</span>
              <input class="input mono" value={groupedSecret} readonly onfocus={(e) => e.currentTarget.select()} />
            </div>
            <div class="field">
              <span class="field-label">Or import this link</span>
              <div style="display:flex; gap: var(--space-2)">
                <input class="input mono" value={setup.uri} readonly onfocus={(e) => e.currentTarget.select()} style="min-width:0; flex:1" />
                <button class="btn btn-secondary" type="button" onclick={copyURI}>{copied ? "Copied" : "Copy"}</button>
              </div>
            </div>
            <label class="field">
              <span class="field-label">Code from the app</span>
              <input class="input mono" bind:value={totpCode} inputmode="numeric" autocomplete="one-time-code" maxlength="7" required />
            </label>
            {#if totpError}
              <div class="notice notice-danger" style="margin:0"><Icon name="warn" size={18} /><span>{totpError}</span></div>
            {/if}
            <div style="display:flex; gap: var(--space-2)">
              <button class="btn btn-primary" type="submit" disabled={totpBusy}>
                {totpBusy ? "Confirming…" : "Turn on"}
              </button>
              <button class="btn btn-ghost" type="button" onclick={cancelTOTPSetup}>Cancel</button>
            </div>
          </form>
        {:else}
          <form onsubmit={startTOTPSetup} style="display:flex; flex-direction:column; gap: var(--space-4)">
            <div style="font-size:13px; color: var(--color-neutral-700); line-height:1.6; margin-top:-6px">
              Require a 6-digit code from an authenticator app at sign-in, on top of the password.
              Turning it on signs out every other session.
            </div>
            <label class="field">
              <span class="field-label">Password</span>
              <input class="input" type="password" bind:value={totpPassword} autocomplete="current-password" required />
            </label>
            {#if totpError}
              <div class="notice notice-danger" style="margin:0"><Icon name="warn" size={18} /><span>{totpError}</span></div>
            {/if}
            <div>
              <button class="btn btn-secondary" type="submit" disabled={totpBusy || !settings}>
                {totpBusy ? "Starting…" : "Set up two-factor"}
              </button>
            </div>
          </form>
        {/if}
      </div>

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
