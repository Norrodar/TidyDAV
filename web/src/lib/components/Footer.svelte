<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { t } from '$lib/i18n';

  const repo = 'Norrodar/TidyDAV';
  const repoUrl = `https://github.com/${repo}`;

  let running = $state('');
  let latest = $state('');
  let latestUrl = $state(repoUrl);

  // Short-form a 40-char commit sha, leave tags/"dev" as-is.
  function short(v: string): string {
    return /^[0-9a-f]{40}$/i.test(v) ? v.slice(0, 7) : v;
  }

  const isSha = $derived(/^[0-9a-f]{40}$/i.test(running));
  const comparable = $derived(Boolean(running && latest && isSha));
  const upToDate = $derived(comparable && short(running) === short(latest));

  onMount(async () => {
    try {
      running = (await api.health()).version;
    } catch {
      /* health unavailable — leave blank */
    }
    try {
      const res = await fetch(`https://api.github.com/repos/${repo}/commits/main`, {
        headers: { Accept: 'application/vnd.github+json' }
      });
      if (res.ok) {
        const data = (await res.json()) as { sha?: string; html_url?: string };
        latest = data.sha ?? '';
        if (data.html_url) latestUrl = data.html_url;
      }
    } catch {
      /* offline or rate-limited — hide the latest-version row */
    }
  });
</script>

<footer class="footer">
  <div class="left">
    <span class="brand">Tidy<span class="accent">DAV</span></span>
    <span class="tagline">{t('footer_tagline')}</span>
  </div>

  <div class="right">
    {#if running}
      <span class="meta">
        {t('footer_running')}:
        <code>{short(running)}</code>
        {#if comparable}
          {#if upToDate}
            <span class="pill ok">{t('footer_up_to_date')}</span>
          {:else}
            <a class="pill warn" href={latestUrl} target="_blank" rel="noreferrer noopener">
              {t('footer_update_available')}
            </a>
          {/if}
        {/if}
      </span>
    {/if}
    {#if latest && !upToDate}
      <span class="meta">
        {t('footer_latest')}:
        <a href={latestUrl} target="_blank" rel="noreferrer noopener"><code>{short(latest)}</code></a>
      </span>
    {/if}
    <a class="meta link" href={repoUrl} target="_blank" rel="noreferrer noopener">{t('footer_source')} ↗</a>
  </div>
</footer>

<style>
  .footer {
    position: relative;
    z-index: 1;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    max-width: 1040px;
    width: 100%;
    margin: 0 auto;
    padding: var(--space-5);
    border-top: 1px solid var(--separator);
    font-size: var(--text-xs);
    color: var(--text-tertiary);
  }
  .left {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .brand {
    font-weight: var(--weight-semibold);
    color: var(--text-secondary);
    letter-spacing: -0.01em;
  }
  .accent {
    color: var(--accent);
  }
  .right {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    flex-wrap: wrap;
  }
  .meta {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
  }
  code {
    color: var(--text-secondary);
  }
  .pill {
    padding: 1px var(--space-2);
    border-radius: var(--radius-full);
    font-weight: var(--weight-medium);
  }
  .pill.ok {
    background: rgba(48, 209, 88, 0.16);
    color: var(--success);
  }
  .pill.warn {
    background: rgba(255, 159, 10, 0.16);
    color: var(--warning);
  }
  .link:hover {
    color: var(--text-secondary);
  }
</style>
