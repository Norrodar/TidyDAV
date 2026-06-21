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
  import Footer from '$lib/components/Footer.svelte';

  let { children } = $props();

  onMount(() => {
    session.refresh();
  });

  // Watermark rows; alternating rows scroll in opposite diagonal directions at a
  // constant, very slow pace.
  const wmRows = Array.from({ length: 9 });
  const wmWords = Array.from({ length: 14 });

  // Apply custom accent color from config when present.
  $effect(() => {
    const color = session.accentColor;
    if (!color) return;
    try {
      const hex = color.replace('#', '');
      const full =
        hex.length === 3 ? hex.split('').map((c) => c + c).join('') : hex;
      const r = parseInt(full.slice(0, 2), 16);
      const g = parseInt(full.slice(2, 4), 16);
      const b = parseInt(full.slice(4, 6), 16);

      document.documentElement.style.setProperty('--accent', color);

      // Hover: lighten each channel by ~30.
      const rh = Math.min(255, r + 30);
      const gh = Math.min(255, g + 30);
      const bh = Math.min(255, b + 30);
      const hover = `#${rh.toString(16).padStart(2, '0')}${gh.toString(16).padStart(2, '0')}${bh.toString(16).padStart(2, '0')}`;
      document.documentElement.style.setProperty('--accent-hover', hover);

      // Relative luminance (WCAG 2.x) — determines readable text color on
      // the accent background. Values > 0.179 (roughly mid-range) need dark
      // text; below that white text is fine.
      const linearize = (c: number) => {
        const s = c / 255;
        return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
      };
      const lum = 0.2126 * linearize(r) + 0.7152 * linearize(g) + 0.0722 * linearize(b);
      const textOnAccent = lum > 0.179 ? '#0a0a0c' : '#ffffff';
      document.documentElement.style.setProperty('--accent-text', textOnAccent);

      document.documentElement.style.setProperty('--focus-ring', `0 0 0 3px ${color}55`);
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
  <div class="wallpaper" aria-hidden="true">
    <div class="wm-field">
      {#each wmRows as _, r}
        <div class="wm-row" class:reverse={r % 2 === 1} style="--d:{-r * 13}s; opacity:{0.92 - (r % 3) * 0.16}">
          <div class="wm-track">
            {#each wmWords as _w}
              <span class="wm-word">Tidy<span class="dav">DAV</span></span>
            {/each}
          </div>
        </div>
      {/each}
    </div>
  </div>

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

  <Footer />
</div>

<Toasts />
<ConfirmDialog />

<style>
  .app {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
  }

  .wallpaper {
    position: fixed;
    inset: 0;
    z-index: 0;
    pointer-events: none;
    overflow: hidden;
  }
  .wm-field {
    position: absolute;
    top: 50%;
    left: 50%;
    width: 200vmax;
    height: 200vmax;
    transform: translate(-50%, -50%) rotate(-45deg);
    display: flex;
    flex-direction: column;
    justify-content: space-around;
  }
  .wm-row {
    overflow: hidden;
    white-space: nowrap;
    /* One whole "TidyDAV " unit in the monospace face: 7 chars + a space gap.
       Translating by exactly this keeps the repeat seamless. */
    font-family: var(--font-mono);
    font-weight: 800;
    font-size: clamp(120px, 16vw, 320px);
  }
  .wm-track {
    display: inline-flex;
    will-change: transform;
    animation: wm-marquee 64s linear infinite;
    animation-delay: var(--d, 0s);
  }
  .wm-row.reverse .wm-track {
    animation-name: wm-marquee-rev;
  }
  .wm-word {
    padding-right: 1ch;
    color: rgba(255, 255, 255, 0.06);
  }
  .wm-word .dav {
    color: var(--accent);
    opacity: 0.13;
  }
  @keyframes wm-marquee {
    from { transform: translateX(0); }
    to { transform: translateX(-8ch); }
  }
  @keyframes wm-marquee-rev {
    from { transform: translateX(-8ch); }
    to { transform: translateX(0); }
  }
  @media (prefers-reduced-motion: reduce) {
    .wm-track { animation: none; }
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
    position: relative;
    z-index: 1;
    flex: 1;
    width: 100%;
    max-width: 1040px;
    margin: 0 auto;
    padding: var(--space-7) var(--space-5);
  }
</style>
