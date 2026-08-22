<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";
  import type { Host, HostMutationResult } from "../types";

  let {
    host,
    onClose,
    onSaved,
  }: {
    host: Host | null;
    onClose: () => void;
    onSaved: (result: HostMutationResult) => void;
  } = $props();

  let domain = $state(host?.domain ?? "");
  let upstream = $state(host?.upstream ?? "");
  let enabled = $state(host?.enabled ?? true);
  let headerPairs = $state<{ key: string; value: string }[]>(
    host?.request_headers
      ? Object.entries(host.request_headers).map(([key, value]) => ({ key, value }))
      : [],
  );
  let error = $state("");
  let submitting = $state(false);

  function addHeader() {
    headerPairs = [...headerPairs, { key: "", value: "" }];
  }

  function removeHeader(index: number) {
    headerPairs = headerPairs.filter((_, i) => i !== index);
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    error = "";
    submitting = true;

    const request_headers: Record<string, string> = {};
    for (const { key, value } of headerPairs) {
      if (key.trim()) request_headers[key.trim()] = value;
    }
    const payload = { domain: domain.trim(), upstream: upstream.trim(), request_headers, enabled };

    try {
      const result = host ? await api.updateHost(host.id, payload) : await api.createHost(payload);
      onSaved(result);
    } catch (err) {
      error = errorMessage(err);
    } finally {
      submitting = false;
    }
  }
</script>

<div
  class="modal-backdrop"
  role="presentation"
  onclick={onClose}
  onkeydown={(e) => e.key === "Escape" && onClose()}
>
  <div
    class="modal"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => e.stopPropagation()}
    onkeydown={(e) => e.stopPropagation()}
  >
    <form onsubmit={submit}>
      <h2>{host ? "Edit proxy host" : "Add proxy host"}</h2>

      <div class="field">
        <label for="domain">Domain</label>
        <input id="domain" bind:value={domain} placeholder="app.example.com" required />
      </div>
      <div class="field">
        <label for="upstream">Forward to</label>
        <input id="upstream" bind:value={upstream} placeholder="http://192.168.1.50:3000" required />
      </div>
      <div class="field">
        <label><input type="checkbox" bind:checked={enabled} /> Enabled</label>
      </div>

      <div class="field">
        <span class="field-label-text">Request headers sent to upstream</span>
        {#each headerPairs as pair, i}
          <div style="display:flex; gap:8px; margin-bottom:6px;">
            <input placeholder="Header" bind:value={pair.key} />
            <input placeholder="Value" bind:value={pair.value} />
            <button type="button" class="secondary" onclick={() => removeHeader(i)}>✕</button>
          </div>
        {/each}
        <button type="button" class="secondary" onclick={addHeader}>+ Add header</button>
      </div>

      {#if error}<div class="error-text">{error}</div>{/if}

      <div class="form-actions">
        <button type="button" class="secondary" onclick={onClose}>Cancel</button>
        <button class="primary" type="submit" disabled={submitting}>Save</button>
      </div>
    </form>
  </div>
</div>
