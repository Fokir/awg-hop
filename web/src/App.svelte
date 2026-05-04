<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from './lib/api';
  import type { SetupStatus } from './lib/types';
  import Bootstrap from './views/Bootstrap.svelte';
  import Login from './views/Login.svelte';
  import Dashboard from './views/Dashboard.svelte';
  import Peers from './views/Peers.svelte';
  import Tunnels from './views/Tunnels.svelte';
  import Settings from './views/Settings.svelte';
  import Backup from './views/Backup.svelte';

  type View = 'dashboard' | 'peers' | 'tunnels' | 'settings' | 'backup';

  let loading = $state(true);
  let setupComplete = $state(false);
  let loggedIn = $state(false);
  let view = $state<View>('dashboard');
  let error = $state<string | null>(null);

  onMount(checkSession);

  async function checkSession() {
    loading = true;
    error = null;
    try {
      const st = await api<SetupStatus>('/api/v1/setup/status');
      setupComplete = st.setup_complete;
      if (!setupComplete) {
        loggedIn = false;
        return;
      }
      try {
        await api('/api/v1/peers');
        loggedIn = true;
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) {
          loggedIn = false;
        } else {
          throw e;
        }
      }
    } catch (e) {
      error = e instanceof ApiError ? e.message : String(e);
    } finally {
      loading = false;
    }
  }

  async function logout() {
    try {
      await api('/api/v1/auth/logout', { method: 'POST' });
    } catch {
      // ignore
    }
    loggedIn = false;
  }

  const tabs: { id: View; label: string }[] = [
    { id: 'dashboard', label: 'Dashboard' },
    { id: 'peers', label: 'Пиры' },
    { id: 'tunnels', label: 'Исходящие туннели' },
    { id: 'settings', label: 'Настройки' },
    { id: 'backup', label: 'Бэкап / Импорт' },
  ];
</script>

{#if loading}
  <div class="page-center">Загрузка…</div>
{:else if !setupComplete}
  <main class="page">
    <Bootstrap onDone={checkSession} />
  </main>
{:else if !loggedIn}
  <Login onDone={checkSession} />
{:else}
  <header class="topbar">
    <div class="brand">AWG Hop</div>
    <nav>
      {#each tabs as t}
        <button
          type="button"
          class:active={view === t.id}
          onclick={() => (view = t.id)}
        >
          {t.label}
        </button>
      {/each}
    </nav>
    <button type="button" class="logout" onclick={logout}>Выйти</button>
  </header>

  <main class="page">
    {#if error}<p class="err">{error}</p>{/if}
    {#if view === 'dashboard'}
      <Dashboard />
    {:else if view === 'peers'}
      <Peers />
    {:else if view === 'tunnels'}
      <Tunnels />
    {:else if view === 'settings'}
      <Settings />
    {:else if view === 'backup'}
      <Backup />
    {/if}
  </main>
{/if}

<style>
  :global(body) {
    margin: 0;
    background: #f3f4f6;
    color: #1f2328;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif;
  }
  :global(code) {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.85em;
  }
  :global(h1, h2, h3) {
    color: #111827;
  }
  .page-center {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #6b7280;
  }
  .page {
    max-width: 1100px;
    margin: 0 auto;
    padding: 1.5rem;
  }
  .topbar {
    background: #111827;
    color: #f9fafb;
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.75rem 1.5rem;
  }
  .brand {
    font-weight: 700;
    letter-spacing: 0.02em;
    font-size: 1.05rem;
  }
  nav {
    display: flex;
    gap: 0.25rem;
    flex: 1;
  }
  nav button {
    background: transparent;
    color: #cbd5e1;
    border: 0;
    padding: 0.45rem 0.85rem;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.95rem;
  }
  nav button.active {
    background: #1f2937;
    color: #fff;
  }
  .logout {
    background: transparent;
    color: #cbd5e1;
    border: 1px solid #374151;
    padding: 0.4rem 0.75rem;
    border-radius: 6px;
    cursor: pointer;
  }
  .err {
    color: #b00020;
  }
</style>
