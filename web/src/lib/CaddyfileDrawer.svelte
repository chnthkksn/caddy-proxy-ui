<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";

  let { onClose }: { onClose: () => void } = $props();

  let text = $state("");
  let loading = $state(true);
  let error = $state("");

  (async () => {
    try {
      text = await api.exportCaddyfile();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      loading = false;
    }
  })();
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && onClose()} />

<div class="drawer-scrim" role="presentation" onclick={onClose}>
  <div
    class="drawer"
    role="dialog"
    aria-modal="true"
    aria-label="Caddyfile"
    tabindex="-1"
    onclick={(e) => e.stopPropagation()}
    onkeydown={(e) => e.stopPropagation()}
  >
    <div class="drawer-head">
      <div class="drawer-title">Caddyfile</div>
      <span class="tag tag-outline" style="color: var(--ink-dim); border-color: color-mix(in srgb, #f9f4ed 30%, transparent)">
        read-only
      </span>
      <button class="drawer-close" onclick={onClose} aria-label="Close">×</button>
    </div>
    <div class="drawer-body">
      {#if loading}
        <pre style="margin:0; font-family: var(--font-mono); font-size:13.5px; color: var(--ink-dim)">Loading…</pre>
      {:else if error}
        <pre style="margin:0; font-family: var(--font-mono); font-size:13.5px; color: #e2725b">{error}</pre>
      {:else if !text.trim()}
        <pre style="margin:0; font-family: var(--font-mono); font-size:13.5px; color: var(--ink-dim)"># No hosts configured yet.</pre>
      {:else}
        <pre style="margin:0; font-family: var(--font-mono); font-size:13.5px; line-height:1.75; color: var(--ink-text); white-space:pre">{text}</pre>
      {/if}
    </div>
  </div>
</div>
