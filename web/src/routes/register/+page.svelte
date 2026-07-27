<script lang="ts">
  import { t } from '$lib/i18n';
  import { goto } from '$app/navigation';
  import { api, ApiError } from '$lib/api';
  import { session } from '$lib/state/session.svelte';

  let email = $state('');
  let password = $state('');
  let submitting = $state(false);
  let error = $state<string | null>(null);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    submitting = true;
    error = null;
    try {
      session.apply(await api.register(email, password));
      await goto('/feeds');
    } catch (err) {
      error = err instanceof ApiError ? err.message : t('save_failed');
    } finally {
      submitting = false;
    }
  }
</script>

<div class="auth">
  <div class="card">
    <h1>{t('create_account')}</h1>
    <p class="subtitle">{t('create_account_sub')}</p>

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
          autocomplete="new-password"
          minlength="8"
          required
        />
      </label>

      {#if error}<p class="error">{error}</p>{/if}

      <button class="button" type="submit" disabled={submitting}>
        {submitting ? t('creating') : t('create_account')}
      </button>
    </form>

    <p class="hint">{t('have_account')} <a href="/login">{t('sign_in')}</a></p>
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
  .button[type='submit'] {
    margin-top: var(--space-2);
  }
  .error {
    margin: 0;
    color: var(--danger);
    font-size: var(--text-sm);
  }
  .hint {
    margin: var(--space-4) 0 0;
    text-align: center;
    color: var(--text-tertiary);
    font-size: var(--text-sm);
  }
</style>
