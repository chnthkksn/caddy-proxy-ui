<script lang="ts">
  import { api } from "../api";
  import { errorMessage } from "../errors";
  import { data } from "../store.svelte";
  import type { AccessRule, Host } from "../types";
  import Icon from "./Icon.svelte";
  import AccessRuleModal from "./AccessRuleModal.svelte";
  import ConfirmDialog from "./ConfirmDialog.svelte";

  let rulesByHost = $state<Record<number, AccessRule[]>>({});
  let loading = $state(true);
  let error = $state("");
  let modalHost = $state<Host | null>(null);
  let pendingDelete = $state<AccessRule | null>(null);

  const kindLabel: Record<AccessRule["kind"], string> = {
    basic_auth: "password",
    ip_allow: "allow list",
    ip_deny: "deny list",
  };

  const kindTag: Record<AccessRule["kind"], string> = {
    basic_auth: "tag-accent",
    ip_allow: "tag-accent-2",
    ip_deny: "tag-danger",
  };

  async function load() {
    loading = true;
    error = "";
    try {
      const entries = await Promise.all(
        data.hosts.map(async (h) => [h.id, await api.listAccessRules(h.id)] as const),
      );
      rulesByHost = Object.fromEntries(entries);
    } catch (err) {
      error = errorMessage(err);
    } finally {
      loading = false;
    }
  }

  function describe(rule: AccessRule): string {
    switch (rule.kind) {
      case "basic_auth":
        return `Username “${rule.value}” — anyone without these credentials gets 401.`;
      case "ip_allow":
        return `${rule.value} — everything else gets 403.`;
      case "ip_deny":
        return `${rule.value} — blocked with 403.`;
    }
  }

  async function removeRule() {
    if (!pendingDelete) return;
    const rule = pendingDelete;
    pendingDelete = null;
    error = "";
    try {
      const result = await api.deleteAccessRule(rule.id);
      data.noteSync(result.sync);
      await load();
    } catch (err) {
      error = errorMessage(err);
    }
  }

  load();
</script>

<div style="display:flex; flex-direction:column; gap: var(--space-4)">
  {#if error}
    <div class="notice notice-danger"><Icon name="warn" size={18} /><span>{error}</span></div>
  {/if}

  {#if loading}
    <div class="panel"><div class="text-muted">Loading…</div></div>
  {:else if data.hosts.length === 0}
    <div class="panel">
      <div class="text-muted">Add a proxy host first — access rules attach to a host.</div>
    </div>
  {:else}
    <div class="grid-forms">
      {#each data.hosts as host (host.id)}
        {@const rules = rulesByHost[host.id] ?? []}
        <div class="panel" style="display:flex; flex-direction:column; gap: var(--space-3)">
          <div style="display:flex; align-items:center; gap: var(--space-2)">
            <div style="min-width:0">
              {#if host.group_label}<div class="kicker">{host.group_label}</div>{/if}
              <div style="font-size:18px; font-weight:700; overflow-wrap:anywhere">{host.domain}</div>
            </div>
            <span class="tag tag-neutral" style="margin-left:auto">
              {rules.length === 0 ? "open" : `${rules.length} rule${rules.length === 1 ? "" : "s"}`}
            </span>
          </div>

          {#if rules.length === 0}
            <div style="font-size:14px; color: var(--color-neutral-700); line-height:1.5">
              No rules — this host is reachable by anyone who can resolve it.
            </div>
          {:else}
            <div style="display:flex; flex-direction:column; gap: var(--space-2)">
              {#each rules as rule (rule.id)}
                <div style="display:flex; align-items:flex-start; gap: var(--space-2)">
                  <span class="tag {kindTag[rule.kind]}" style="flex:none; margin-top:2px">
                    {kindLabel[rule.kind]}
                  </span>
                  <div class="mono" style="font-size:13px; color: var(--color-neutral-700); flex:1; min-width:0; overflow-wrap:anywhere">
                    {describe(rule)}
                  </div>
                  <button
                    class="btn btn-ghost btn-danger"
                    onclick={() => (pendingDelete = rule)}
                    aria-label="Remove rule"
                    title="Remove rule"
                    style="flex:none; padding:2px 8px"
                  >
                    <Icon name="trash" size={14} />
                  </button>
                </div>
              {/each}
            </div>
          {/if}

          <div style="border-top:1px solid var(--color-divider); padding-top: var(--space-3)">
            <button class="btn btn-secondary" onclick={() => (modalHost = host)} style="font-size:13px">
              + Add rule
            </button>
          </div>
        </div>
      {/each}
    </div>
  {/if}

  <div class="prose">
    Rules run before the request reaches your upstream — an IP block never touches the app behind it.
    Basic-auth passwords are bcrypt-hashed on the way into SQLite and are never shown again.
  </div>
</div>

{#if modalHost}
  <AccessRuleModal
    host={modalHost}
    onClose={() => (modalHost = null)}
    onSaved={(result) => {
      modalHost = null;
      data.noteSync(result.sync);
      load();
    }}
  />
{/if}

{#if pendingDelete}
  <ConfirmDialog
    title="Remove this access rule?"
    body="The rule is deleted and Caddy's config is rebuilt without it, so the host stops enforcing it right away."
    confirmLabel="Remove rule"
    danger
    onConfirm={removeRule}
    onCancel={() => (pendingDelete = null)}
  />
{/if}
