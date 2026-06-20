<script lang="ts">
  import { goto } from '$app/navigation';
  import { api, ApiError } from '$lib/api';
  import { session } from '$lib/state/session.svelte';
  import { t, tSignInWith } from '$lib/i18n';

  let email = $state('');
  let password = $state('');
  let submitting = $state(false);
  let error = $state<string | null>(null);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    submitting = true;
    error = null;
    try {
      session.apply(await api.login(email, password));
      await goto('/');
    } catch (err) {
      error = err instanceof ApiError ? err.message : t('sign_in') + ' failed';
    } finally {
      submitting = false;
    }
  }
</script>

<div class="auth">
  <div class="card">
    <h1>{t('sign_in')}</h1>
    <p class="subtitle">{t('welcome_back')}</p>

    {#if !session.oidcOnly}
      <form onsubmit={submit}>
        <label>
          <span>{t('email')}</span>
          <input class="input" type="email" bind:value={email} autocomplete="username" required />
        </label>
        <label>
          <span>{t('password')}</span>
          <input
            class="input"
            type="password"
            bind:value={password}
            autocomplete="current-password"
            required
          />
        </label>

        {#if error}<p class="error">{error}</p>{/if}

        <button class="button submit-btn" type="submit" disabled={submitting}>
          {submitting ? t('signing_in') : t('sign_in')}
        </button>
      </form>
    {/if}

    {#if session.oidcEnabled}
      {#if !session.oidcOnly}
        <div class="divider"><span>{t('or')}</span></div>
      {/if}

      <a class="button sso-btn" class:button-secondary={!session.oidcOnly} href="/auth/oidc/login">
        {tSignInWith(session.oidcDisplayName)}
      </a>
    {/if}

    {#if session.registrationEnabled}
      <p class="hint">{t('no_account')} <a href="/register">{t('create_one')}</a></p>
    {/if}
    {#if session.mailEnabled && !session.oidcOnly}
      <p class="hint"><a href="/reset/request">{t('forgot_password')}</a></p>
    {/if}
  </div>
</div>

<style>
  .auth {
    display: flex;
    justify-content: center;
    padding-top: var(--space-7);
  }

  .card {
    width: 100%;
    max-width: 380px;
  }

  h1 {
    font-size: var(--text-2xl);
  }

  .subtitle {
    margin: var(--space-2) 0 var(--space-6);
    color: var(--text-secondary);
    font-size: var(--text-sm);
  }

  form {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  label {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-secondary);
  }

  .submit-btn {
    margin-top: var(--space-2);
    width: 100%;
  }

  .sso-btn {
    width: 100%;
    margin-top: var(--space-2);
  }

  .error {
    margin: 0;
    color: var(--danger);
    font-size: var(--text-sm);
  }

  .divider {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    margin: var(--space-5) 0;
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

  .hint {
    margin: var(--space-4) 0 0;
    text-align: center;
    color: var(--text-tertiary);
    font-size: var(--text-sm);
  }
</style>
