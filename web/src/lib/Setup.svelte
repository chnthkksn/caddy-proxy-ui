<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";
  import { theme } from "../store.svelte";
  import Icon from "./Icon.svelte";

  let { onDone }: { onDone: () => void } = $props();

  let username = $state("");
  let password = $state("");
  let confirmPassword = $state("");
  let error = $state("");
  let submitting = $state(false);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    error = "";
    submitting = true;
    try {
      await api.setup(username, password, confirmPassword);
      onDone();
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
        <div class="auth-title">Create your account</div>
        <div class="auth-sub">
          One administrator account, chosen by you. There are no default credentials to forget to
          change.
        </div>
      </div>

      <label class="field">
        <span class="field-label">Username</span>
        <input class="input" bind:value={username} autocomplete="username" required />
      </label>
      <label class="field">
        <span class="field-label">Password</span>
        <input class="input" type="password" bind:value={password} minlength="8" autocomplete="new-password" required />
        <span class="field-hint">At least 8 characters.</span>
      </label>
      <label class="field">
        <span class="field-label">Confirm password</span>
        <input class="input" type="password" bind:value={confirmPassword} minlength="8" autocomplete="new-password" required />
      </label>

      {#if error}
        <div class="notice notice-danger" style="margin:0"><Icon name="warn" size={18} /><span>{error}</span></div>
      {/if}

      <button class="btn btn-primary btn-block" type="submit" disabled={submitting}>
        {submitting ? "Creating…" : "Create account"}
      </button>
    </form>
  </div>
</div>
