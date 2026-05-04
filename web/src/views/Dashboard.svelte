<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import { formatBytes, formatHandshake, isFreshHandshake } from '../lib/format';
  import type { Client, UpstreamTunnel, SystemStatus } from '../lib/types';

  let clients = $state<Client[]>([]);
  let upstreams = $state<UpstreamTunnel[]>([]);
  let status = $state<SystemStatus | null>(null);
  let error = $state<string | null>(null);
  let applyMsg = $state<string | null>(null);
  let applying = $state(false);

  let timer: number | null = null;

  onMount(() => {
    refresh();
    timer = window.setInterval(refresh, 10000);
    return () => {
      if (timer !== null) window.clearInterval(timer);
    };
  });

  async function refresh() {
    try {
      [clients, upstreams, status] = await Promise.all([
        api<Client[]>('/api/v1/clients'),
        api<UpstreamTunnel[]>('/api/v1/upstreams'),
        api<SystemStatus>('/api/v1/system/status'),
      ]);
      error = null;
    } catch (e) {
      error = e instanceof ApiError ? e.message : String(e);
    }
  }

  async function apply() {
    applying = true;
    applyMsg = null;
    try {
      await api('/api/v1/system/apply', { method: 'POST' });
      applyMsg = 'Apply успешно применён.';
      await refresh();
    } catch (e) {
      applyMsg = e instanceof ApiError ? `Apply ошибка: ${e.message}` : String(e);
    } finally {
      applying = false;
    }
  }

  const enabledClients = $derived(clients.filter((c) => c.enabled));
  const onlineClients = $derived(
    enabledClients.filter((c) => isFreshHandshake(c.status?.latest_handshake_unix ?? 0)),
  );
  const enabledUpstreams = $derived(upstreams.filter((t) => t.enabled));
  const upUpstreams = $derived(
    enabledUpstreams.filter((t) => isFreshHandshake(t.status?.latest_handshake_unix ?? 0)),
  );
</script>

<div class="row apply-row">
  <button type="button" onclick={apply} disabled={applying}>
    {applying ? 'Применяется…' : 'Применить (Apply)'}
  </button>
  {#if applyMsg}<span>{applyMsg}</span>{/if}
</div>

{#if error}<p class="err">{error}</p>{/if}

<div class="cards">
  <div class="card">
    <div class="num">{onlineClients.length} / {enabledClients.length}</div>
    <div class="label">Активные клиенты (handshake &lt; 3 мин)</div>
  </div>
  <div class="card">
    <div class="num">{upUpstreams.length} / {enabledUpstreams.length}</div>
    <div class="label">Активные upstream-подключения</div>
  </div>
  <div class="card">
    <div class="num">{status?.policy_routing?.backend ?? '—'}</div>
    <div class="label">Backend</div>
    {#if status?.policy_routing?.last_error}
      <small class="err">{status.policy_routing.last_error}</small>
    {/if}
  </div>
</div>

<section>
  <h2>Топ-5 клиентов по обмену</h2>
  <table>
    <thead>
      <tr>
        <th>Имя</th>
        <th>Handshake</th>
        <th>RX</th>
        <th>TX</th>
      </tr>
    </thead>
    <tbody>
      {#each [...clients].sort((a, b) => (b.status?.transfer_tx_bytes ?? 0) - (a.status?.transfer_tx_bytes ?? 0)).slice(0, 5) as c (c.id)}
        <tr>
          <td>{c.name}</td>
          <td>{c.status?.latest_handshake_unix ? formatHandshake(c.status.latest_handshake_unix) : '—'}</td>
          <td>{c.status ? formatBytes(c.status.transfer_rx_bytes) : '—'}</td>
          <td>{c.status ? formatBytes(c.status.transfer_tx_bytes) : '—'}</td>
        </tr>
      {/each}
      {#if clients.length === 0}
        <tr><td colspan="4" class="muted center">Клиентов пока нет.</td></tr>
      {/if}
    </tbody>
  </table>
</section>

<section>
  <h2>Upstream-подключения</h2>
  <table>
    <thead>
      <tr>
        <th>Имя</th>
        <th>Интерфейс</th>
        <th>Включён</th>
        <th>Handshake</th>
        <th>RX / TX</th>
      </tr>
    </thead>
    <tbody>
      {#each upstreams as t (t.id)}
        <tr>
          <td>{t.name}</td>
          <td><code>{t.interface_name}</code></td>
          <td>{t.enabled ? 'да' : 'нет'}</td>
          <td>
            {#if t.status?.latest_handshake_unix}
              {formatHandshake(t.status.latest_handshake_unix)}
            {:else if t.status?.last_error}
              <span class="err" title={t.status.last_error}>ошибка</span>
            {:else}
              <span class="muted">—</span>
            {/if}
          </td>
          <td>
            {#if t.status}
              {formatBytes(t.status.transfer_rx_bytes)} / {formatBytes(t.status.transfer_tx_bytes)}
            {:else}
              <span class="muted">—</span>
            {/if}
          </td>
        </tr>
      {/each}
      {#if upstreams.length === 0}
        <tr><td colspan="5" class="muted center">Upstream-подключений пока нет.</td></tr>
      {/if}
    </tbody>
  </table>
</section>

<style>
  .row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-bottom: 1rem;
  }
  .apply-row button {
    padding: 0.5rem 1rem;
    border: 0;
    border-radius: 6px;
    background: #2563eb;
    color: #fff;
    cursor: pointer;
    font-weight: 600;
  }
  .apply-row button[disabled] {
    opacity: 0.6;
  }
  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 1rem;
    margin-bottom: 1.5rem;
  }
  .card {
    background: #fff;
    border-radius: 12px;
    padding: 1.25rem 1.5rem;
    border: 1px solid #e5e7eb;
  }
  .card .num {
    font-size: 1.75rem;
    font-weight: 700;
  }
  .card .label {
    color: #6b7280;
    font-size: 0.9rem;
  }
  section {
    background: #fff;
    border-radius: 12px;
    padding: 1.25rem 1.5rem;
    border: 1px solid #e5e7eb;
    margin-bottom: 1.5rem;
  }
  h2 {
    margin-top: 0;
  }
  table {
    width: 100%;
    border-collapse: collapse;
  }
  th,
  td {
    padding: 0.5rem 0.6rem;
    text-align: left;
    border-bottom: 1px solid #f3f4f6;
  }
  th {
    font-size: 0.78rem;
    text-transform: uppercase;
    color: #6b7280;
  }
  .muted {
    color: #9ca3af;
  }
  .center {
    text-align: center;
    padding: 1.25rem;
  }
  .err {
    color: #b00020;
  }
</style>
