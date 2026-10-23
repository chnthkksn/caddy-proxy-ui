<script lang="ts">
  import { api, ApiError } from "../api";
  import { errorMessage } from "../errors";
  import { data, theme } from "../store.svelte";
  import Icon from "./Icon.svelte";

  let { onLoggedIn }: { onLoggedIn: () => void } = $props();

  let username = $state("");
  let password = $state("");
  let error = $state("");
  let submitting = $state(false);

  // Set once the password checks out and the account has two-factor on.
  let challenge = $state("");
  let code = $state("");

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    error = "";
    submitting = true;
    try {
      const res = await api.login(username, password);
      if (res.totp_required && res.challenge) {
        challenge = res.challenge;
        return;
      }
      onLoggedIn();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      submitting = false;
    }
  }

  async function submitCode(e: SubmitEvent) {
    e.preventDefault();
    error = "";
    submitting = true;
    try {
      await api.loginTOTP(challenge, code);
      onLoggedIn();
    } catch (err) {
      error = errorMessage(err);
      code = "";
      if (err instanceof ApiError && err.status === 410) backToPassword(error);
    } finally {
      submitting = false;
    }
  }

  function backToPassword(message = "") {
    challenge = "";
    code = "";
    password = "";
    error = message;
  }
</script>

<div class="auth" data-theme={theme.value}>
  <div class="auth-inner">
    <div class="auth-brand">
      <div class="brand-mark"><Icon name="shield" size={22} /></div>
      <div class="brand-name">caddy-ui</div>
    </div>

    {#if challenge}
    <form class="auth-card" onsubmit={submitCode}>
      <div>
        <div class="auth-title">Two-factor code</div>
        <div class="auth-sub">Enter the 6-digit code from your authenticator app.</div>
      </div>

      <label class="field">
        <span class="field-label">Authentication code</span>
        <!-- svelte-ignore a11y_autofocus -->
        <input
          class="input mono"
          bind:value={code}
          inputmode="numeric"
          autocomplete="one-time-code"
          pattern="[0-9 ]*"
          maxlength="7"
          autofocus
          required
        />
      </label>

      {#if error}
        <div class="notice notice-danger" style="margin:0"><Icon name="warn" size={18} /><span>{error}</span></div>
      {/if}

      <button class="btn btn-primary btn-block" type="submit" disabled={submitting}>
        {submitting ? "Verifying…" : "Verify"}
      </button>
      <button class="btn btn-ghost btn-block" type="button" onclick={() => backToPassword()}>Back</button>

      <div class="auth-foot">
        Lost your device? <span class="mono">caddy-ui reset-password</span> on the server turns two-factor off.
      </div>
    </form>
    {:else}
    <form class="auth-card" onsubmit={submit}>
      <div>
        <div class="auth-title">Sign in</div>
        <div class="auth-sub">One administrator account. No default credentials.</div>
      </div>

      <label class="field">
        <span class="field-label">Username</span>
        <input class="input" bind:value={username} autocomplete="username" required />
      </label>
      <label class="field">
        <span class="field-label">Password</span>
        <input class="input" type="password" bind:value={password} autocomplete="current-password" required />
      </label>

      {#if error}
        <div class="notice notice-danger" style="margin:0"><Icon name="warn" size={18} /><span>{error}</span></div>
      {/if}

      <button class="btn btn-primary btn-block" type="submit" disabled={submitting}>
        {submitting ? "Signing in…" : "Sign in"}
      </button>

      <div class="auth-foot">
        Forgot it? <span class="mono">caddy-ui reset-password</span> on the server.
      </div>
    </form>
    {/if}

    <div class="auth-foot">
      {data.status.version ? `v${data.status.version}` : "caddy-ui"}
    </div>
  </div>
</div>
