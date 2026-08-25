<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";
  import { data } from "../store.svelte";
  import type { ImportResult } from "../types";
  import Icon from "./Icon.svelte";

  let text = $state("");
  let loading = $state(true);
  let error = $state("");

  let importOpen = $state(false);
  let importText = $state("");
  let importing = $state(false);
  let result = $state<ImportResult | null>(null);

  async function load() {
    loading = true;
    error = "";
    try {
      text = await api.exportCaddyfile();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      loading = false;
    }
  }

  function download() {
    const blob = new Blob([text], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "Caddyfile";
    a.click();
    URL.revokeObjectURL(url);
  }

  async function runImport() {
    importing = true;
    error = "";
    result = null;
    try {
      result = await api.importCaddyfile(importText);
      data.noteSync(result.sync);
      importText = "";
      await data.loadAll();
      await load();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      importing = false;
    }
  }

  load();
</script>

<div style="display:flex; flex-direction:column; gap: var(--space-4)">
  {#if error}
    <div class="notice notice-danger"><Icon name="warn" size={18} /><span>{error}</span></div>
  {/if}

  <div style="display:flex; gap: var(--space-2); flex-wrap:wrap; align-items:center">
    <span class="tag tag-outline">
      generated from {data.hostCount} host{data.hostCount === 1 ? "" : "s"}
    </span>
    <button class="btn btn-secondary" onclick={() => (importOpen = !importOpen)} style="margin-left:auto; font-size:14px">
      {importOpen ? "Close import" : "Import Caddyfile"}
    </button>
    <button class="btn btn-secondary" onclick={download} disabled={!text} style="font-size:14px">
      <Icon name="download" size={16} />
      Download
    </button>
  </div>

  {#if importOpen}
    <div class="panel" style="display:flex; flex-direction:column; gap: var(--space-3)">
      <div class="panel-title">Import</div>
      <div style="font-size:14px; color: var(--color-neutral-700); line-height:1.6">
        Paste a Caddyfile. Only simple one-domain reverse-proxy blocks are imported; anything else
        comes back listed as skipped, never dropped quietly.
      </div>
      <textarea
        class="input"
        bind:value={importText}
        placeholder={"app.example.com {\n\treverse_proxy localhost:3000\n}"}
        rows="8"
      ></textarea>
      <div style="display:flex; gap: var(--space-2)">
        <button class="btn btn-primary" onclick={runImport} disabled={importing || !importText.trim()}>
          {importing ? "Importing…" : "Import hosts"}
        </button>
      </div>

      {#if result}
        <div class="notice" style="margin:0">
          <Icon name="warn" size={18} />
          <span>
            Imported {result.imported} host{result.imported === 1 ? "" : "s"}.
            {#if result.skipped.length > 0}
              Skipped {result.skipped.length}:
              {result.skipped.map((s) => `${s.domain} (${s.reason})`).join("; ")}
            {/if}
          </span>
        </div>
      {/if}
    </div>
  {/if}

  <div class="ink">
    {#if loading}
      <pre style="color: var(--ink-dim)">Loading…</pre>
    {:else if !text.trim()}
      <pre style="color: var(--ink-dim)"># No hosts configured yet.</pre>
    {:else}
      <pre>{text}</pre>
    {/if}
  </div>

  <div class="prose">
    This is generated from what's in SQLite, which is the source of truth — it's the config Caddy is
    given, not a file on disk you can edit. Once you'd rather hand-write your own Caddyfile, download
    this as a starting point and this UI stops owning it.
  </div>
</div>
