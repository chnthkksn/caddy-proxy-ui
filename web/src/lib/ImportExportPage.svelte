<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";
  import type { ImportResult } from "../types";

  let { onChange }: { onChange?: () => void } = $props();

  const placeholder = "app.example.com {\n    reverse_proxy 10.0.0.5:3000\n}";

  let caddyfileText = $state("");
  let importing = $state(false);
  let result = $state<ImportResult | null>(null);
  let error = $state("");

  async function onFileChange(e: Event) {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    caddyfileText = await file.text();
  }

  async function doImport() {
    error = "";
    result = null;
    importing = true;
    try {
      result = await api.importCaddyfile(caddyfileText);
      onChange?.();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      importing = false;
    }
  }
</script>

<div class="content">
  <h2>Import / Export</h2>

  <div class="card" style="padding:20px; margin-bottom:20px;">
    <h3 style="margin-top:0;">Import a Caddyfile</h3>
    <p class="muted">
      Only simple "domain {'{'} reverse_proxy upstream {'}'}" blocks are imported automatically.
      Anything more complex is listed as skipped so nothing is silently dropped.
    </p>
    <div class="field">
      <input type="file" accept=".caddyfile,text/plain" onchange={onFileChange} />
    </div>
    <div class="field">
      <textarea rows="8" bind:value={caddyfileText} placeholder={placeholder}></textarea>
    </div>
    <button class="primary" onclick={doImport} disabled={importing || !caddyfileText.trim()}>
      Import
    </button>

    {#if error}<div class="banner error" style="margin-top:16px;">{error}</div>{/if}

    {#if result}
      <div class="banner {result.skipped.length > 0 ? 'warn' : 'success'}" style="margin-top:16px;">
        Imported {result.imported}, skipped {result.skipped.length}.
      </div>
      {#if result.skipped.length > 0}
        <ul>
          {#each result.skipped as s}
            <li><strong>{s.domain}</strong> — {s.reason}</li>
          {/each}
        </ul>
      {/if}
    {/if}
  </div>

  <div class="card" style="padding:20px;">
    <h3 style="margin-top:0;">Export</h3>
    <p class="muted">Download all proxy hosts as a clean, hand-editable Caddyfile.</p>
    <a class="secondary" style="text-decoration:none; display:inline-block;" href="/api/export">
      Download Caddyfile
    </a>
  </div>
</div>
