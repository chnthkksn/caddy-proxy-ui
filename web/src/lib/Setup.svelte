<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";

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

<div class="auth-shell">
  <form class="card auth-card" onsubmit={submit}>
    <h1>Create administrator account</h1>
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
        autocomplete="new-password"
        required
        minlength="8"
      />
    </div>
    <div class="field">
      <label for="confirm">Confirm password</label>
      <input
        id="confirm"
        type="password"
        bind:value={confirmPassword}
        autocomplete="new-password"
        required
        minlength="8"
      />
    </div>
    {#if error}<div class="error-text">{error}</div>{/if}
    <div class="form-actions">
      <button class="primary" type="submit" disabled={submitting}>Create account</button>
    </div>
  </form>
</div>
