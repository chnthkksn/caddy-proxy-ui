<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";
  import type { AccessRuleKind, AccessRuleMutationResult, Host } from "../types";
  import Icon from "./Icon.svelte";

  let {
    host,
    onClose,
    onSaved,
  }: {
    host: Host;
    onClose: () => void;
    onSaved: (result: AccessRuleMutationResult) => void;
  } = $props();

  let kind = $state<AccessRuleKind>("basic_auth");
  let username = $state("");
  let password = $state("");
  let ipValue = $state("");
  let error = $state("");
  let saving = $state(false);

  const hints: Record<AccessRuleKind, string> = {
    basic_auth: "The browser prompts for these credentials before anything reaches your upstream.",
    ip_allow: "Only these addresses get through. Everything else gets 403.",
    ip_deny: "These addresses are blocked with 403. Everyone else is unaffected.",
  };

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    error = "";
    saving = true;
    try {
      const result =
        kind === "basic_auth"
          ? await api.addBasicAuthRule(host.id, username.trim(), password)
          : await api.addIPRule(host.id, kind, ipValue.trim());
      onSaved(result);
    } catch (err) {
      error = errorMessage(err);
    } finally {
      saving = false;
    }
  }
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && onClose()} />

<div class="scrim" role="presentation" onclick={onClose}>
  <div
    class="modal"
    role="dialog"
    aria-modal="true"
    aria-label="Add access rule"
    tabindex="-1"
    onclick={(e) => e.stopPropagation()}
    onkeydown={(e) => e.stopPropagation()}
  >
    <form onsubmit={submit} style="display:flex; flex-direction:column; gap: var(--space-4)">
      <div>
        <div class="modal-title">Add access rule</div>
        <div style="font-size:14px; color: var(--color-neutral-700); margin-top:4px">
          for {host.domain}
        </div>
      </div>

      <div class="field">
        <span class="field-label">Rule type</span>
        <div class="seg" style="width:100%">
          <button type="button" class="seg-opt" class:is-on={kind === "basic_auth"} onclick={() => (kind = "basic_auth")} style="flex:1; justify-content:center">
            Password
          </button>
          <button type="button" class="seg-opt" class:is-on={kind === "ip_allow"} onclick={() => (kind = "ip_allow")} style="flex:1; justify-content:center">
            Allow IPs
          </button>
          <button type="button" class="seg-opt" class:is-on={kind === "ip_deny"} onclick={() => (kind = "ip_deny")} style="flex:1; justify-content:center">
            Deny IPs
          </button>
        </div>
        <span class="field-hint">{hints[kind]}</span>
      </div>

      {#if kind === "basic_auth"}
        <label class="field">
          <span class="field-label">Username</span>
          <input class="input" bind:value={username} required />
        </label>
        <label class="field">
          <span class="field-label">Password</span>
          <input class="input" type="password" bind:value={password} minlength="8" required />
          <span class="field-hint">At least 8 characters. Stored as a bcrypt hash, never in plain text.</span>
        </label>
      {:else}
        <label class="field">
          <span class="field-label">IP address or CIDR range</span>
          <input class="input" bind:value={ipValue} placeholder="203.0.113.0/24" required />
          <span class="field-hint">A single address (198.51.100.7) or a range (203.0.113.0/24).</span>
        </label>
      {/if}

      {#if error}
        <div class="notice notice-danger" style="margin:0"><Icon name="warn" size={18} /><span>{error}</span></div>
      {/if}

      <div class="modal-actions">
        <button class="btn btn-ghost" type="button" onclick={onClose}>Cancel</button>
        <button class="btn btn-primary" type="submit" disabled={saving}>
          {saving ? "Adding…" : "Add rule"}
        </button>
      </div>
    </form>
  </div>
</div>
