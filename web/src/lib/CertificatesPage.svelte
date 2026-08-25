<script lang="ts">
  import { data, daysUntil, renewalPhrase } from "../store.svelte";
  import Icon from "./Icon.svelte";

  let certs = $derived(data.certs);

  function stateFor(notAfter: string): { label: string; cls: string } {
    const days = daysUntil(notAfter);
    if (days < 0) return { label: "expired", cls: "tag-danger" };
    if (days <= 7) return { label: "renewing soon", cls: "tag-accent" };
    return { label: "valid", cls: "tag-accent-2" };
  }
</script>

<div style="display:flex; flex-direction:column; gap: var(--space-4)">
  {#if certs && !certs.available}
    <div class="notice">
      <Icon name="warn" size={18} />
      <span>
        <strong>Certificate storage isn't readable from here.</strong> Either Caddy uses a custom
        storage backend, or caddy-ui hasn't been granted read access to its data directory.
      </span>
    </div>
  {:else if certs}
    <div class="panel panel-flush" style="overflow-x:auto">
      <table class="table" style="min-width:560px">
        <thead>
          <tr>
            <th>Domain</th>
            <th>Issuer</th>
            <th>Renews</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {#each certs.certificates as cert (cert.domain)}
            {@const s = stateFor(cert.not_after)}
            <tr>
              <td style="font-weight:600">{cert.domain}</td>
              <td style="color: var(--color-neutral-700)">{cert.issuer}</td>
              <td class="mono" style="color: var(--color-neutral-700); font-size:13px">
                {renewalPhrase(cert.not_after)}
              </td>
              <td><span class="tag {s.cls}">{s.label}</span></td>
            </tr>
          {/each}
        </tbody>
      </table>

      {#if certs.certificates.length === 0}
        <div style="padding: var(--space-6) 0; color: var(--color-neutral-600); font-size:14px">
          No certificates issued yet. Caddy requests one the first time a live host is reached over
          HTTPS.
        </div>
      {/if}
    </div>
  {/if}

  <div class="prose">
    Certificates are stored and renewed by Caddy itself, in its own data directory. There is nothing
    to upload, no cron job and no ACME client to keep alive — this page only reads what Caddy already
    knows.
  </div>
</div>
