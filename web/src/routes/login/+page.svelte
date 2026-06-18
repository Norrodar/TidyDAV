<script lang="ts">
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
      session.apply(await api.login(email, password));
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Login failed';
    } finally {
      submitting = false;
    }
  }
</script>

<div class="auth">
  <div class="card">
    <h1>Sign in</h1>
    <p class="subtitle">Welcome back.</p>

    <form onsubmit={submit}>
      <label>
        <span>Email</span>
        <input class="input" type="email" bind:value={email} autocomplete="username" required />
      </label>
      <label>
        <span>Password</span>
        <input
          class="input"
          type="password"
          bind:value={password}
          autocomplete="current-password"
          required
        />
      </label>

      {#if error}<p class="error">{error}</p>{/if}

      <button class="button" type="submit" disabled={submitting}>
        {submitting ? 'Signing in…' : 'Sign in'}
      </button>
    </form>

    <div class="divider"><span>or</span></div>

    <a class="button button-secondary" href="/auth/oidc/login">Continue with SSO</a>
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
</style>
