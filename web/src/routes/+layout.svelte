<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import '$lib/styles/global.css';
  import { api } from '$lib/api';
  import { session } from '$lib/state/session.svelte';
  import Toasts from '$lib/components/Toasts.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';

  let { children } = $props();

  onMount(() => {
    session.refresh();
  });

  async function logout() {
    try {
      await api.logout();
    } catch {
      /* ignore */
    }
    await session.refresh();
    await goto('/');
  }
</script>

<div class="app">
  <header class="topbar">
    <a class="brand" href="/">Tidy<span class="brand-accent">DAV</span></a>
    <nav class="nav">
      {#if session.authenticated}
        <a href="/feeds">Feeds</a>
        <a href="/sync">Sync</a>
        {#if session.user?.isAdmin}<a href="/audit">Audit</a>{/if}
        <button class="linklike" onclick={logout}>Sign out</button>
      {:else}
        <a href="/login">Sign in</a>
      {/if}
    </nav>
  </header>
  <main class="content">
    {@render children()}
  </main>
</div>

<Toasts />
<ConfirmDialog />

<style>
  .app {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
  }

  .topbar {
    position: sticky;
    top: 0;
    z-index: 10;
    display: flex;
    align-items: center;
    height: 56px;
    padding: 0 var(--space-5);
    border-bottom: 1px solid var(--separator);
    background: var(--bg-overlay);
    backdrop-filter: blur(var(--blur));
    -webkit-backdrop-filter: blur(var(--blur));
  }

  .brand {
    font-size: var(--text-lg);
    font-weight: var(--weight-semibold);
    letter-spacing: -0.02em;
    color: var(--text-primary);
  }

  .brand-accent {
    color: var(--accent);
  }

  .nav {
    display: flex;
    gap: var(--space-4);
    margin-left: var(--space-6);
  }

  .nav a {
    color: var(--text-secondary);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
  }

  .nav a:hover {
    color: var(--text-primary);
  }

  .linklike {
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    color: var(--text-secondary);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
  }

  .linklike:hover {
    color: var(--text-primary);
  }

  .content {
    flex: 1;
    width: 100%;
    max-width: 960px;
    margin: 0 auto;
    padding: var(--space-7) var(--space-5);
  }
</style>
