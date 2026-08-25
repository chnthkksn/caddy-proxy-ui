<script lang="ts">
  import { untrack } from "svelte";
  import { api } from "../api";
  import { errorMessage } from "../errors";
  import { data } from "../store.svelte";
  import type { Host } from "../types";
  import Icon from "./Icon.svelte";
  import ConfirmDialog from "./ConfirmDialog.svelte";

  let { host, onDone }: { host: Host | null; onDone: () => void } = $props();

  // Seeding form state from the prop is deliberate: App keys this component
  // on the host id, so a different host remounts it rather than mutating a
  // half-edited form underneath the user.
  const initial = untrack(() => host);

  let domain = $state(initial?.domain ?? "");
  let upstream = $state(initial?.upstream ?? "");
  let groupLabel = $state(initial?.group_label ?? "");
  let enabled = $state(initial?.enabled ?? true);
  let headerPairs = $state<{ key: string; value: string }[]>(
    initial?.request_headers
      ? Object.entries(initial.request_headers).map(([key, value]) => ({ key, value }))
      : [],
  );

  let error = $state("");
  let saving = $state(false);
  let confirmDelete = $state(false);

  // Mirrors internal/caddyconfig.NormalizeDial — the DB keeps the upstream as
  // typed, but Caddy dials a bare host:port.
  function normalizeDial(raw: string): string {
    return raw.replace(/^https?:\/\//, "").replace(/\/$/, "");
  }

  // The same shape internal/caddyfile.Export writes, so the preview is what
  // the Caddyfile page will actually show — not an illustration.
  let snippet = $derived.by(() => {
    const d = domain.trim() || "app.example.com";
    const u = normalizeDial(upstream.trim()) || "localhost:3000";
    const lines = [`${d} {`];
    const headers = headerPairs.filter((p) => p.key.trim());
    if (headers.length > 0) {
      lines.push(`\treverse_proxy ${u} {`);
      for (const { key, value } of headers) lines.push(`\t\theader_up ${key.trim()} ${value}`);
      lines.push("\t}");
    } else {
      lines.push(`\treverse_proxy ${u}`);
    }
    lines.push("}");
    return lines.join("\n");
  });

  function addHeader() {
    headerPairs = [...headerPairs, { key: "", value: "" }];
  }

  function removeHeader(index: number) {
    headerPairs = headerPairs.filter((_, i) => i !== index);
  }

  async function save(e: SubmitEvent) {
    e.preventDefault();
    error = "";
    saving = true;

    const request_headers: Record<string, string> = {};
    for (const { key, value } of headerPairs) {
      if (key.trim()) request_headers[key.trim()] = value;
    }
    const payload = {
      domain: domain.trim(),
      upstream: upstream.trim(),
      request_headers,
      group_label: groupLabel.trim(),
      enabled,
    };

    try {
      const result = host ? await api.updateHost(host.id, payload) : await api.createHost(payload);
      data.noteSync(result.sync);
      await data.loadAll();
      onDone();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      saving = false;
    }
  }

  async function remove() {
    if (!host) return;
    confirmDelete = false;
    error = "";
    try {
      const result = await api.deleteHost(host.id);
      data.noteSync(result.sync);
      await data.loadAll();
      onDone();
    } catch (err) {
      error = errorMessage(err);
    }
  }
</script>

<form onsubmit={save}>
  <div class="grid-forms">
    <div class="panel" style="display:flex; flex-direction:column; gap: var(--space-4)">
      {#if error}
        <div class="notice notice-danger" style="margin:0"><Icon name="warn" size={18} /><span>{error}</span></div>
      {/if}

      <label class="field">
        <span class="field-label">Domain</span>
        <input class="input" bind:value={domain} placeholder="app.client.com" required />
      </label>

      <label class="field">
        <span class="field-label">Upstream</span>
        <input class="input" bind:value={upstream} placeholder="localhost:3000" required />
        <span class="field-hint">Host:port, or a full URL — the scheme is stripped before Caddy dials it.</span>
      </label>

      <label class="field">
        <span class="field-label">Client / label</span>
        <input class="input" bind:value={groupLabel} placeholder="Northwind" />
        <span class="field-hint">Optional. Groups hosts in the list and is searchable.</span>
      </label>

      <div
        style="display:flex; align-items:center; justify-content:space-between; gap: var(--space-3); background: var(--color-neutral-200); border-radius: var(--radius-md); padding: var(--space-3) var(--space-4)"
      >
        <div>
          <div style="font-size:14px; font-weight:600">Enabled</div>
          <div style="font-size:13px; color: var(--color-neutral-700)">
            Disabled hosts stay saved but leave the config
          </div>
        </div>
        <button
          type="button"
          class="switch"
          class:is-on={enabled}
          onclick={() => (enabled = !enabled)}
          role="switch"
          aria-checked={enabled}
          aria-label="Enabled"
        >
          <span class="knob"></span>
        </button>
      </div>

      <div style="display:flex; gap: var(--space-2); flex-wrap:wrap; align-items:center; border-top:1px solid var(--color-divider); padding-top: var(--space-4)">
        <button class="btn btn-primary" type="submit" disabled={saving}>
          {saving ? "Saving…" : "Save & reload Caddy"}
        </button>
        <button class="btn btn-ghost" type="button" onclick={onDone}>Cancel</button>
        {#if host}
          <button class="btn btn-ghost btn-danger" type="button" onclick={() => (confirmDelete = true)} style="margin-left:auto">
            Delete host
          </button>
        {/if}
      </div>
    </div>

    <div style="display:flex; flex-direction:column; gap: var(--space-4)">
      <div
        style="background: var(--color-accent-2-200); border-radius: var(--radius-lg); padding: var(--space-6); display:flex; gap: var(--space-3); align-items:flex-start; color: var(--color-accent-2-900)"
      >
        <span style="flex:none; margin-top:2px; color: var(--color-accent-2-800)"><Icon name="certificates" size={20} /></span>
        <div>
          <div style="font-weight:700; font-size:15px">HTTPS is not a step</div>
          <div style="font-size:13px; margin-top:4px; line-height:1.5">
            Caddy issues and renews the certificate the moment this host goes live, and redirects
            HTTP for you. Nothing to configure here.
          </div>
        </div>
      </div>

      <details class="panel" style="padding: var(--space-4) var(--space-6)">
        <summary style="cursor:pointer; font-weight:700; font-size:15px; padding: var(--space-2) 0">
          Advanced
          <span style="font-weight:400; color: var(--color-neutral-600); font-size:13px">— request headers</span>
        </summary>
        <div style="display:flex; flex-direction:column; gap: var(--space-3); padding-top: var(--space-3)">
          {#each headerPairs as pair, i (i)}
            <div style="display:grid; grid-template-columns: 1fr 1fr auto; gap: var(--space-2)">
              <input class="input" placeholder="X-Forwarded-Host" bind:value={pair.key} style="min-width:0" />
              <input class="input" placeholder="{'{host}'}" bind:value={pair.value} style="min-width:0" />
              <button class="btn btn-ghost btn-danger" type="button" onclick={() => removeHeader(i)} aria-label="Remove header">
                <Icon name="trash" size={15} />
              </button>
            </div>
          {/each}
          <button class="btn btn-ghost" type="button" onclick={addHeader} style="align-self:flex-start; font-size:13px">
            + Add header
          </button>
          <div style="font-size:13px; color: var(--color-neutral-600); line-height:1.5">
            Websockets and streaming pass through by default. Everything else lives in the Caddyfile.
          </div>
        </div>
      </details>

      <div class="ink">
        <div class="ink-kicker">This host becomes</div>
        <pre style="white-space:pre-wrap">{snippet}</pre>
      </div>
    </div>
  </div>
</form>

{#if confirmDelete && host}
  <ConfirmDialog
    title={`Delete ${host.domain}?`}
    body="The host is removed from SQLite and dropped from Caddy's config on the next push. Its certificate stays in Caddy's storage until it expires."
    confirmLabel="Delete host"
    danger
    onConfirm={remove}
    onCancel={() => (confirmDelete = false)}
  />
{/if}
