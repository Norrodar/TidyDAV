<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import '$lib/styles/global.css';
  import { api } from '$lib/api';
  import { session } from '$lib/state/session.svelte';
  import { t } from '$lib/i18n';
  import Toasts from '$lib/components/Toasts.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';

  let { children } = $props();

  onMount(() => {
    session.refresh();
  });

  // Apply custom accent color from config when present.
  $effect(() => {
    const color = session.accentColor;
    if (!color) return;
    document.documentElement.style.setProperty('--accent', color);
    // Lighten by ~30 per channel for hover state.
    try {
      const hex = color.replace('#', '');
      const full = hex.length === 3
        ? hex.split('').map((c) => c + c).join('')
        : hex;
      const r = Math.min(255, parseInt(full.slice(0, 2), 16) + 30);
      const g = Math.min(255, parseInt(full.slice(2, 4), 16) + 30);
      const b = Math.min(255, parseInt(full.slice(4, 6), 16) + 30);
      const hover = `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`;
      document.documentElement.style.setProperty('--accent-hover', hover);
      document.documentElement.style.setProperty(
        '--focus-ring',
        `0 0 0 3px ${color}55`
      );
    } catch {
      // Non-critical — ignore parse errors.
    }
  });

  // Central auth guard.
  $effect(() => {
    if (session.loading) return;
    const path = $page.url.pathname;
    const isProtected =
      path.startsWith('/feeds') || path.startsWith('/sync') || path.startsWith('/audit');
    if (isProtected && !session.authenticated) {
      goto('/login');
    } else if (path.startsWith('/audit') && !session.user?.isAdmin) {
      goto('/feeds');
    }
  });

  async function logout() {
    if (session.oidcEnabled) {
      // Let the server clear the session and redirect to the OIDC end_session_endpoint.
      window.location.href = '/auth/oidc/logout';
    } else {
      try {
        await api.logout();
      } catch {
        /* ignore */
      }
      await session.refresh();
      await goto('/');
    }
  }

  // Derive initials / avatar for the nav.
  const avatarLabel = $derived(() => {
    const u = session.user;
    if (!u) return '';
    if (u.email) return u.email[0].toUpperCase();
    return 'U';
  });
</script>

<div class="app">
  <header class="topbar">
    <a class="brand" href="/">Tidy<span class="brand-accent">DAV</span></a>

    {#if session.authenticated}
      <nav class="nav">
        <a href="/feeds">{t('nav_feeds')}</a>
        <a href="/sync">{t('nav_sync')}</a>
        {#if session.user?.isAdmin}<a href="/audit">{t('nav_audit')}</a>{/if}
      </nav>
    {/if}

    <div class="nav-right">
      {#if session.authenticated}
        <button class="linklike sign-out" onclick={logout}>{t('sign_out')}</button>
        <div class="avatar" title={session.user?.email ?? ''}>
          {#if session.user?.avatarUrl}
            <img src={session.user.avatarUrl} alt="" />
          {:else}
            <span>{avatarLabel()}</span>
          {/if}
        </div>
      {:else if !session.loading}
        <a class="button button-sm" href="/login">{t('sign_in')}</a>
      {/if}
    </div>
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
    gap: var(--space-5);
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
    flex-shrink: 0;
  }

  .brand-accent {
    color: var(--accent);
  }

  .nav {
    display: flex;
    gap: var(--space-4);
  }

  .nav a {
    color: var(--text-secondary);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    transition: color var(--dur-fast) var(--ease);
  }

  .nav a:hover {
    color: var(--text-primary);
  }

  /* Push everything after the nav to the right. */
  .nav-right {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .sign-out {
    color: var(--text-secondary);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
  }

  .sign-out:hover {
    color: var(--text-primary);
  }

  .avatar {
    width: 32px;
    height: 32px;
    border-radius: var(--radius-full);
    background: var(--accent);
    color: var(--accent-text);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    overflow: hidden;
    flex-shrink: 0;
  }

  .avatar img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .linklike {
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
  }

  .button-sm {
    padding: var(--space-2) var(--space-4);
    font-size: var(--text-sm);
  }

  .content {
    flex: 1;
    width: 100%;
    max-width: 1040px;
    margin: 0 auto;
    padding: var(--space-7) var(--space-5);
  }
</style>
