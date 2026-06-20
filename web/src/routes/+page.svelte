<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type HealthResponse } from '$lib/api';
  import { session } from '$lib/state/session.svelte';

  let status = $state<'checking' | 'ok' | 'error'>('checking');
  let health = $state<HealthResponse | null>(null);

  onMount(async () => {
    try {
      health = await api.health();
      status = 'ok';
    } catch {
      status = 'error';
    }
  });
</script>

<section class="hero">
  <h1>Welcome to TidyDAV</h1>
  <p class="subtitle">Your calendars and contacts, tidied up.</p>
</section>

<div class="card">
  <div class="row">
    <span>Backend</span>
    {#if status === 'checking'}
      <span class="badge">checking…</span>
    {:else if status === 'ok'}
      <span class="badge badge-ok">online{#if health?.version}&nbsp;· v{health.version}{/if}</span>
    {:else}
      <span class="badge badge-error">unreachable</span>
    {/if}
  </div>
  {#if session.authenticated}
    <p class="hint">Manage your transformed calendars and DAV sync jobs.</p>
    <div class="actions">
      <a class="button" href="/feeds">Feeds</a>
      <a class="button button-secondary" href="/sync">DAV sync</a>
    </div>
  {:else}
    <p class="hint">Sign in to manage your feeds and sync jobs.</p>
    <a class="button" href="/login">Sign in</a>
  {/if}
</div>

<style>
  .hero {
    margin-bottom: var(--space-6);
  }

  .hero h1 {
    font-size: var(--text-3xl);
  }

  .subtitle {
    margin: var(--space-2) 0 0;
    color: var(--text-secondary);
    font-size: var(--text-lg);
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-4);
  }

  .hint {
    margin: 0 0 var(--space-5);
    color: var(--text-tertiary);
    font-size: var(--text-sm);
  }

  .actions {
    display: flex;
    gap: var(--space-3);
  }
</style>
