<script lang="ts">
  import { session } from '$lib/state/session.svelte';
  import { t, tSignInWith } from '$lib/i18n';
</script>

{#if session.authenticated}
  <!-- Dashboard: large feature tiles -->
  <div class="dashboard">
    <h1 class="dash-title">TidyDAV</h1>

    <div class="tiles">
      <a class="tile tile-feeds" href="/feeds">
        <div class="tile-bg">
          <!-- Decorative calendar grid -->
          <svg viewBox="0 0 200 200" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
            <rect x="20" y="30" width="160" height="150" rx="8" fill="none" stroke="currentColor" stroke-width="3" opacity="0.18"/>
            <line x1="20" y1="60" x2="180" y2="60" stroke="currentColor" stroke-width="2" opacity="0.14"/>
            <rect x="20" y="30" width="160" height="30" rx="8" fill="currentColor" opacity="0.1"/>
            {#each [0,1,2,3,4,5,6] as col}
              {#each [0,1,2,3] as row}
                <rect
                  x={30 + col * 22}
                  y={72 + row * 26}
                  width="14"
                  height="14"
                  rx="2"
                  fill="currentColor"
                  opacity={Math.random() > 0.6 ? 0.18 : 0.06}
                />
              {/each}
            {/each}
          </svg>
        </div>
        <div class="tile-content">
          <div class="tile-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3" y="4" width="18" height="18" rx="2"/>
              <line x1="16" y1="2" x2="16" y2="6"/>
              <line x1="8" y1="2" x2="8" y2="6"/>
              <line x1="3" y1="10" x2="21" y2="10"/>
            </svg>
          </div>
          <h2>{t('home_feeds_title')}</h2>
          <p>{t('home_feeds_desc')}</p>
          <span class="tile-cta">{t('home_open')} →</span>
        </div>
      </a>

      <a class="tile tile-sync" href="/sync">
        <div class="tile-bg">
          <!-- Decorative sync arrows -->
          <svg viewBox="0 0 200 200" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
            <circle cx="100" cy="100" r="70" fill="none" stroke="currentColor" stroke-width="3" opacity="0.12"/>
            <circle cx="100" cy="100" r="45" fill="none" stroke="currentColor" stroke-width="2" opacity="0.1"/>
            <path d="M60 80 Q100 50 140 80" fill="none" stroke="currentColor" stroke-width="3" opacity="0.2"/>
            <path d="M140 120 Q100 150 60 120" fill="none" stroke="currentColor" stroke-width="3" opacity="0.2"/>
            <polygon points="140,72 148,84 132,84" fill="currentColor" opacity="0.2"/>
            <polygon points="60,128 52,116 68,116" fill="currentColor" opacity="0.2"/>
          </svg>
        </div>
        <div class="tile-content">
          <div class="tile-icon tile-icon-sync">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/>
              <path d="M3 3v5h5"/>
              <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/>
              <path d="M16 16h5v5"/>
            </svg>
          </div>
          <h2>{t('home_sync_title')}</h2>
          <p>{t('home_sync_desc')}</p>
          <span class="tile-cta">{t('home_open')} →</span>
        </div>
      </a>
    </div>
  </div>

{:else}
  <!-- Landing page for unauthenticated visitors -->
  <div class="landing">
    <div class="hero">
      <div class="wordmark">Tidy<span class="accent">DAV</span></div>
      <p class="tagline">{t('home_headline')}</p>
    </div>

    <div class="features">
      <div class="feature">
        <div class="feature-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="4" width="18" height="18" rx="2"/>
            <line x1="16" y1="2" x2="16" y2="6"/>
            <line x1="8" y1="2" x2="8" y2="6"/>
            <line x1="3" y1="10" x2="21" y2="10"/>
          </svg>
        </div>
        <div>
          <h3>{t('home_feeds_title')}</h3>
          <p>{t('home_feeds_desc')}</p>
        </div>
      </div>

      <div class="feature">
        <div class="feature-icon feature-icon-sync">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/>
            <path d="M3 3v5h5"/>
            <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/>
            <path d="M16 16h5v5"/>
          </svg>
        </div>
        <div>
          <h3>{t('home_sync_title')}</h3>
          <p>{t('home_sync_desc')}</p>
        </div>
      </div>
    </div>

    <div class="login-section">
      {#if session.oidcEnabled}
        <a class="button sso-primary" href="/auth/oidc/login">
          {tSignInWith(session.oidcDisplayName)}
        </a>
      {/if}

      {#if session.oidcEnabled && !session.oidcOnly}
        <div class="divider"><span>{t('or')}</span></div>
      {/if}

      {#if !session.oidcOnly}
        <a class="button button-secondary" href="/login">{t('home_signin_email')}</a>
      {/if}
    </div>
  </div>
{/if}

<style>
  /* ── Dashboard ────────────────────────────────────────────────────────────── */

  .dashboard {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .dash-title {
    font-size: var(--text-3xl);
    letter-spacing: -0.03em;
  }

  .tiles {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-5);
  }

  @media (max-width: 640px) {
    .tiles {
      grid-template-columns: 1fr;
    }
  }

  .tile {
    position: relative;
    display: flex;
    flex-direction: column;
    min-height: 280px;
    border-radius: var(--radius-lg);
    border: 1px solid var(--separator);
    overflow: hidden;
    text-decoration: none;
    color: var(--text-primary);
    backdrop-filter: blur(32px) brightness(0.82) saturate(120%);
    -webkit-backdrop-filter: blur(32px) brightness(0.82) saturate(120%);
    transition:
      border-color var(--dur-base) var(--ease),
      transform var(--dur-base) var(--ease);
  }

  .tile:hover {
    border-color: rgba(255, 255, 255, 0.2);
    transform: translateY(-2px);
  }

  .tile-feeds {
    background: linear-gradient(145deg, rgba(22, 22, 24, 0.5) 0%, rgba(10, 132, 255, 0.16) 100%);
    color: #0a84ff;
  }

  .tile-sync {
    background: linear-gradient(145deg, rgba(22, 22, 24, 0.5) 0%, rgba(48, 209, 88, 0.14) 100%);
    color: #30d158;
  }

  .tile-bg {
    position: absolute;
    right: -10px;
    bottom: -10px;
    width: 180px;
    height: 180px;
    pointer-events: none;
  }

  .tile-content {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-6);
    flex: 1;
    z-index: 1;
  }

  .tile-icon {
    width: 40px;
    height: 40px;
    border-radius: var(--radius-md);
    background: rgba(10, 132, 255, 0.15);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--accent);
  }

  .tile-icon svg {
    width: 20px;
    height: 20px;
  }

  .tile-icon-sync {
    background: rgba(48, 209, 88, 0.12);
    color: var(--success);
  }

  .tile h2 {
    font-size: var(--text-xl);
    color: var(--text-primary);
  }

  .tile p {
    font-size: var(--text-sm);
    color: var(--text-secondary);
    line-height: 1.6;
    margin: 0;
    flex: 1;
  }

  .tile-cta {
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    color: var(--accent);
  }

  .tile-sync .tile-cta {
    color: var(--success);
  }

  /* ── Landing ──────────────────────────────────────────────────────────────── */

  .landing {
    max-width: 560px;
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-7);
    padding-top: var(--space-6);
  }

  .hero {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .wordmark {
    font-size: var(--text-3xl);
    font-weight: var(--weight-semibold);
    letter-spacing: -0.03em;
    color: var(--text-primary);
  }

  .accent {
    color: var(--accent);
  }

  .tagline {
    font-size: var(--text-lg);
    color: var(--text-secondary);
    margin: 0;
  }

  .features {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  .feature {
    display: flex;
    gap: var(--space-4);
    align-items: flex-start;
  }

  .feature-icon {
    width: 40px;
    height: 40px;
    border-radius: var(--radius-md);
    background: rgba(10, 132, 255, 0.12);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--accent);
    flex-shrink: 0;
  }

  .feature-icon svg {
    width: 20px;
    height: 20px;
  }

  .feature-icon-sync {
    background: rgba(48, 209, 88, 0.1);
    color: var(--success);
  }

  .feature h3 {
    font-size: var(--text-base);
    font-weight: var(--weight-semibold);
    color: var(--text-primary);
    margin-bottom: var(--space-2);
  }

  .feature p {
    font-size: var(--text-sm);
    color: var(--text-secondary);
    line-height: 1.65;
    margin: 0;
  }

  .login-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding-top: var(--space-2);
  }

  .sso-primary {
    width: 100%;
    padding: var(--space-4) var(--space-5);
    font-size: var(--text-base);
  }

  .button-secondary {
    width: 100%;
    padding: var(--space-3) var(--space-5);
  }

  .divider {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    color: var(--text-tertiary);
    font-size: var(--text-xs);
  }

  .divider::before,
  .divider::after {
    content: '';
    flex: 1;
    height: 1px;
    background: var(--separator);
  }
</style>
