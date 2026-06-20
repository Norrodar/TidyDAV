<script lang="ts">
  import { confirmDialog } from '$lib/state/confirm.svelte';

  function onKeydown(event: KeyboardEvent) {
    if (confirmDialog.open && event.key === 'Escape') confirmDialog.cancel();
  }
</script>

<svelte:window onkeydown={onKeydown} />

{#if confirmDialog.open}
  <div class="overlay">
    <button class="overlay-bg" aria-label="Cancel" onclick={() => confirmDialog.cancel()}></button>
    <div class="dialog" role="dialog" aria-modal="true">
      <p class="message">{confirmDialog.message}</p>
      <div class="actions">
        <button class="button button-secondary" onclick={() => confirmDialog.cancel()}>Cancel</button>
        <button class="button danger" onclick={() => confirmDialog.confirm()}>
          {confirmDialog.confirmLabel}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 200;
    display: grid;
    place-items: center;
    padding: var(--space-4);
  }
  .overlay-bg {
    position: absolute;
    inset: 0;
    border: none;
    cursor: default;
    background: rgba(0, 0, 0, 0.5);
  }
  .dialog {
    position: relative;
    width: 100%;
    max-width: 360px;
    padding: var(--space-5);
    background: var(--bg-elevated);
    border: 1px solid var(--separator);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-md);
  }
  .message {
    margin: 0 0 var(--space-5);
    color: var(--text-primary);
    font-size: var(--text-sm);
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-3);
  }
  .danger {
    background: var(--danger);
  }
  .danger:hover {
    background: var(--danger);
    filter: brightness(1.1);
  }
</style>
