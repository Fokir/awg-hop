<script lang="ts">
  import { api, ApiError } from '../lib/api';

  type Props = { onDone: () => void };
  const { onDone }: Props = $props();

  let password = $state('');
  let error = $state<string | null>(null);
  let busy = $state(false);

  async function submit() {
    busy = true;
    error = null;
    try {
      await api('/api/v1/auth/login', { body: { password } });
      onDone();
    } catch (e) {
      if (e instanceof ApiError && e.status === 429) {
        error = 'Слишком много попыток. Подождите немного.';
      } else {
        error = 'Неверный пароль';
      }
    } finally {
      busy = false;
    }
  }
</script>

<div class="login">
  <h1>AWG Hop</h1>
  <form
    onsubmit={(e) => {
      e.preventDefault();
      submit();
    }}
  >
    <label>
      Пароль администратора
      <input type="password" bind:value={password} required />
    </label>
    {#if error}<p class="err">{error}</p>{/if}
    <button type="submit" disabled={busy}>{busy ? 'Вход…' : 'Войти'}</button>
  </form>
</div>

<style>
  .login {
    max-width: 360px;
    margin: 4rem auto;
    padding: 2rem;
    border: 1px solid #e5e7eb;
    border-radius: 12px;
    background: #fff;
  }
  h1 {
    margin-top: 0;
    text-align: center;
  }
  label {
    display: block;
    margin-bottom: 1rem;
  }
  input {
    display: block;
    width: 100%;
    box-sizing: border-box;
    margin-top: 0.25rem;
    padding: 0.5rem 0.75rem;
    border: 1px solid #c8ccd1;
    border-radius: 6px;
  }
  .err {
    color: #b00020;
  }
  button {
    width: 100%;
    padding: 0.6rem 1rem;
    border: 0;
    border-radius: 6px;
    background: #2563eb;
    color: #fff;
    font-weight: 600;
    cursor: pointer;
  }
  button[disabled] {
    opacity: 0.6;
  }
</style>
