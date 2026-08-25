<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";
  import { data, theme } from "../store.svelte";
  import Icon from "./Icon.svelte";

  let { onLoggedIn }: { onLoggedIn: () => void } = $props();

  let username = $state("");
  let password = $state("");
  let error = $state("");
  let submitting = $state(false);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    error = "";
    submitting = true;
    try {
      await api.login(username, password);
      onLoggedIn();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      submitting = false;
    }
  }
</script>

<div class="auth" data-theme={theme.value}>
  <div class="auth-inner">
    <div class="auth-brand">
      <div class="brand-mark"><Icon name="shield" size={22} /></div>
      <div class="brand-name">caddy-ui</div>
    </div>

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

    <div class="auth-foot">
      {data.status.version ? `v${data.status.version}` : "caddy-ui"}
    </div>
  </div>
</div>
