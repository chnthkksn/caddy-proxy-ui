<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";

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

<div class="auth-shell">
  <form class="card auth-card" onsubmit={submit}>
    <h1>Caddy Proxy UI</h1>
    <div class="field">
      <label for="username">Username</label>
      <input id="username" bind:value={username} autocomplete="username" required />
    </div>
    <div class="field">
      <label for="password">Password</label>
      <input
        id="password"
        type="password"
        bind:value={password}
        autocomplete="current-password"
        required
      />
    </div>
    {#if error}<div class="error-text">{error}</div>{/if}
    <div class="form-actions">
      <button class="primary" type="submit" disabled={submitting}>Log in</button>
    </div>
  </form>
</div>
