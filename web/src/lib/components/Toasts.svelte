<script lang="ts">
  import { fly } from 'svelte/transition';
  import { toasts } from '$lib/state/toasts.svelte';
</script>

<div class="toasts" aria-live="polite">
  {#each toasts.items as toast (toast.id)}
    <button
      class="toast {toast.kind}"
      onclick={() => toasts.dismiss(toast.id)}
      transition:fly={{ y: -8, duration: 200 }}
    >
      {toast.message}
    </button>
  {/each}
</div>

<style>
  .toasts {
    position: fixed;
    top: var(--space-5);
    right: var(--space-5);
    z-index: 100;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    pointer-events: none;
  }
  .toast {
    pointer-events: auto;
    cursor: pointer;
    text-align: left;
    max-width: 320px;
    padding: var(--space-3) var(--space-4);
    border: 1px solid var(--separator);
    border-left: 3px solid var(--text-tertiary);
    border-radius: var(--radius-md);
    background: var(--bg-overlay);
    backdrop-filter: blur(var(--blur));
    -webkit-backdrop-filter: blur(var(--blur));
    box-shadow: var(--shadow-md);
    color: var(--text-primary);
    font-size: var(--text-sm);
  }
  .toast.success {
    border-left-color: var(--success);
  }
  .toast.error {
    border-left-color: var(--danger);
  }
  .toast.info {
    border-left-color: var(--accent);
  }
</style>
