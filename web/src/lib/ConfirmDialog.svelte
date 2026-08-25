<script lang="ts">
  let {
    title,
    body,
    confirmLabel = "Confirm",
    danger = false,
    onConfirm,
    onCancel,
  }: {
    title: string;
    body: string;
    confirmLabel?: string;
    danger?: boolean;
    onConfirm: () => void;
    onCancel: () => void;
  } = $props();
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && onCancel()} />

<div class="scrim" role="presentation" onclick={onCancel}>
  <div
    class="modal"
    role="dialog"
    aria-modal="true"
    aria-label={title}
    tabindex="-1"
    onclick={(e) => e.stopPropagation()}
    onkeydown={(e) => e.stopPropagation()}
  >
    <div class="modal-title">{title}</div>
    <div style="font-size:14px; color: var(--color-neutral-700); line-height:1.6">{body}</div>
    <div class="modal-actions">
      <button class="btn btn-ghost" onclick={onCancel}>Cancel</button>
      <button class="btn" class:btn-primary={!danger} class:btn-secondary={danger} class:btn-danger={danger} onclick={onConfirm}>
        {confirmLabel}
      </button>
    </div>
  </div>
</div>
