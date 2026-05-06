<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import { formatBytes, formatHandshake, isFreshHandshake } from '../lib/format';
  import type { Client, UpstreamTunnel, UpstreamType } from '../lib/types';

  let clients = $state<Client[]>([]);
  let upstreams = $state<UpstreamTunnel[]>([]);
  let error = $state<string | null>(null);

  let showForm = $state(false);
  let editing = $state<Client | null>(null);

  let formName = $state('');
  let formAllowedIps = $state('');
  let formPrivateKey = $state('');
  let formPublicKey = $state('');
  let formGeneratePsk = $state(false);
  let formUpstreamType = $state<UpstreamType>('direct');
  let formUpstreamTunnelId = $state<number | ''>('');
  let formEnabled = $state(true);

  onMount(refresh);

  let refreshTimer: number | null = null;
  $effect(() => {
    refreshTimer = window.setInterval(refresh, 15000);
    return () => {
      if (refreshTimer !== null) window.clearInterval(refreshTimer);
    };
  });

  async function refresh() {
    try {
      [clients, upstreams] = await Promise.all([
        api<Client[]>('/api/v1/clients'),
        api<UpstreamTunnel[]>('/api/v1/upstreams'),
      ]);
      error = null;
    } catch (e) {
      error = e instanceof ApiError ? e.message : String(e);
    }
  }

  function openCreate() {
    editing = null;
    formName = '';
    formAllowedIps = '';
    formPrivateKey = '';
    formPublicKey = '';
    formGeneratePsk = false;
    formUpstreamType = 'direct';
    formUpstreamTunnelId = '';
    formEnabled = true;
    showForm = true;
  }

  function openEdit(c: Client) {
    editing = c;
    formName = c.name;
    formAllowedIps = c.allowed_ips;
    formPrivateKey = '';
    formPublicKey = c.public_key;
    formGeneratePsk = false;
    formUpstreamType = c.upstream_type;
    formUpstreamTunnelId = c.upstream_tunnel_id ?? '';
    formEnabled = c.enabled;
    showForm = true;
  }

  async function submit() {
    error = null;
    try {
      if (editing) {
        const body: Record<string, unknown> = {
          name: formName,
          allowed_ips: formAllowedIps,
          enabled: formEnabled,
          upstream_type: formUpstreamType,
          upstream_tunnel_id: formUpstreamType === 'via_upstream' ? Number(formUpstreamTunnelId) : null,
        };
        await api(`/api/v1/clients/${editing.id}`, { method: 'PATCH', body });
      } else {
        const body: Record<string, unknown> = {
          name: formName,
          allowed_ips: formAllowedIps || undefined,
          private_key: formPrivateKey || undefined,
          public_key: formPublicKey || undefined,
          generate_psk: formGeneratePsk,
          upstream_type: formUpstreamType,
          upstream_tunnel_id: formUpstreamType === 'via_upstream' ? Number(formUpstreamTunnelId) : undefined,
        };
        await api('/api/v1/clients', { body });
      }
      showForm = false;
      await refresh();
    } catch (e) {
      error = e instanceof ApiError ? `${e.code}: ${e.message}` : String(e);
    }
  }

  async function toggle(c: Client) {
    try {
      await api(`/api/v1/clients/${c.id}/${c.enabled ? 'disable' : 'enable'}`, { method: 'POST' });
      await refresh();
    } catch (e) {
      error = e instanceof ApiError ? e.message : String(e);
    }
  }

  async function remove(c: Client) {
    if (!window.confirm(`Удалить клиента ${c.name}?`)) return;
    try {
      await api(`/api/v1/clients/${c.id}`, { method: 'DELETE' });
      await refresh();
    } catch (e) {
      error = e instanceof ApiError ? e.message : String(e);
    }
  }

  function upstreamName(id: number | null | undefined) {
    if (!id) return '';
    return upstreams.find((t) => t.id === id)?.name ?? `#${id}`;
  }

  function connStatus(c: Client): 'online' | 'offline' | 'never' {
    if (!c.status?.latest_handshake_unix) return 'never';
    return isFreshHandshake(c.status.latest_handshake_unix) ? 'online' : 'offline';
  }

  const connStatusTitle: Record<string, string> = {
    online: 'Онлайн',
    offline: 'Оффлайн',
    never: 'Никогда не подключался',
  };
</script>

<section>
  <header class="row">
    <h2>Клиенты</h2>
    <button type="button" onclick={openCreate}>+ Новый клиент</button>
  </header>
  {#if error}<p class="err">{error}</p>{/if}

  <table>
    <thead>
      <tr>
        <th>Имя</th>
        <th>Адрес</th>
        <th>Маршрут</th>
        <th>Handshake</th>
        <th>RX / TX</th>
        <th>Статус</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
      {#each clients as c (c.id)}
        <tr class:disabled={!c.enabled}>
          <td>
            <span class="name-cell">
              <span class="dot dot-{connStatus(c)}" title={connStatusTitle[connStatus(c)]}></span>
              {c.name}
            </span>
          </td>
          <td><code>{c.allowed_ips}</code></td>
          <td>
            {#if c.upstream_type === 'direct'}
              <span class="badge direct">прямой выход</span>
            {:else}
              <span class="badge tunnel">→ {upstreamName(c.upstream_tunnel_id)}</span>
            {/if}
          </td>
          <td>
            {#if c.status?.latest_handshake_unix}
              <span class:fresh={isFreshHandshake(c.status.latest_handshake_unix)}>
                {formatHandshake(c.status.latest_handshake_unix)}
              </span>
            {:else}
              <span class="muted">—</span>
            {/if}
          </td>
          <td>
            {#if c.status}
              <small>{formatBytes(c.status.transfer_rx_bytes)} / {formatBytes(c.status.transfer_tx_bytes)}</small>
            {:else}
              <span class="muted">—</span>
            {/if}
          </td>
          <td>
            <button type="button" class="link" onclick={() => toggle(c)}>
              {c.enabled ? 'выкл' : 'вкл'}
            </button>
          </td>
          <td class="actions">
            <a href={`/api/v1/clients/${c.id}/config`}>conf</a>
            <a href={`/api/v1/clients/${c.id}/qrcode`} target="_blank" rel="noreferrer">QR</a>
            <button type="button" class="link" onclick={() => openEdit(c)}>править</button>
            <button type="button" class="link danger" onclick={() => remove(c)}>удалить</button>
          </td>
        </tr>
      {/each}
      {#if clients.length === 0}
        <tr><td colspan="7" class="muted center">Клиентов пока нет.</td></tr>
      {/if}
    </tbody>
  </table>
</section>

{#if showForm}
  <div
    class="modal-bg"
    onclick={() => (showForm = false)}
    onkeydown={(e) => e.key === 'Escape' && (showForm = false)}
    role="presentation"
  >
    <div
      class="modal"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      tabindex="-1"
    >
      <h3>{editing ? `Клиент «${editing.name}»` : 'Новый клиент'}</h3>
      <form
        onsubmit={(e) => {
          e.preventDefault();
          submit();
        }}
      >
        <label>
          Имя
          <input bind:value={formName} required />
        </label>

        <label>
          AllowedIPs ({editing ? 'смена адреса требует Apply' : 'оставьте пустым для авто-выбора'})
          <input bind:value={formAllowedIps} placeholder="10.8.0.5/32" />
        </label>

        {#if !editing}
          <details>
            <summary>Ключи (по умолчанию генерируются)</summary>
            <label>Private key (base64)<input bind:value={formPrivateKey} /></label>
            <label>Public key (base64)<input bind:value={formPublicKey} /></label>
            <label class="row">
              <input type="checkbox" bind:checked={formGeneratePsk} />
              Сгенерировать PresharedKey
            </label>
          </details>
        {/if}

        <fieldset>
          <legend>Куда направить трафик</legend>
          <label class="row">
            <input type="radio" bind:group={formUpstreamType} value="direct" />
            Прямой выход в интернет (NAT внешнего интерфейса)
          </label>
          <label class="row">
            <input type="radio" bind:group={formUpstreamType} value="via_upstream" />
            Через одно из upstream-подключений (наш сервер как AWG-клиент)
          </label>
          {#if formUpstreamType === 'via_upstream'}
            <select bind:value={formUpstreamTunnelId} required>
              <option value="" disabled>— выберите upstream —</option>
              {#each upstreams as t}
                <option value={t.id} disabled={!t.enabled}>{t.name} ({t.interface_name})</option>
              {/each}
            </select>
          {/if}
        </fieldset>

        {#if editing}
          <label class="row">
            <input type="checkbox" bind:checked={formEnabled} />
            Включён
          </label>
        {/if}

        {#if error}<p class="err">{error}</p>{/if}
        <div class="actions">
          <button type="button" onclick={() => (showForm = false)}>Отмена</button>
          <button type="submit">{editing ? 'Сохранить' : 'Создать'}</button>
        </div>
      </form>
    </div>
  </div>
{/if}

<style>
  section {
    background: #fff;
    border-radius: 12px;
    padding: 1.25rem 1.5rem;
    border: 1px solid #e5e7eb;
  }
  header.row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 1rem;
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
    vertical-align: middle;
  }
  th {
    font-size: 0.78rem;
    text-transform: uppercase;
    color: #6b7280;
    font-weight: 600;
  }
  tr.disabled {
    opacity: 0.5;
  }
  td.actions a,
  td.actions button {
    margin-right: 0.5rem;
  }
  .badge {
    padding: 0.1rem 0.5rem;
    border-radius: 999px;
    font-size: 0.75rem;
    font-weight: 600;
  }
  .badge.direct {
    background: #ecfdf5;
    color: #047857;
  }
  .badge.tunnel {
    background: #eff6ff;
    color: #1d4ed8;
  }
  .muted {
    color: #9ca3af;
  }
  .center {
    text-align: center;
    padding: 1.5rem;
  }
  .fresh {
    color: #047857;
    font-weight: 600;
  }
  button {
    padding: 0.4rem 0.75rem;
    border: 1px solid #d1d5db;
    background: #fff;
    border-radius: 6px;
    cursor: pointer;
  }
  button.link {
    border: 0;
    background: none;
    color: #1d4ed8;
    padding: 0;
    cursor: pointer;
  }
  button.link.danger {
    color: #b00020;
  }
  .err {
    color: #b00020;
  }
  .modal-bg {
    position: fixed;
    inset: 0;
    background: rgba(15, 23, 42, 0.45);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 50;
  }
  .modal {
    background: #fff;
    border-radius: 12px;
    padding: 1.5rem;
    width: 520px;
    max-width: 95vw;
    max-height: 90vh;
    overflow: auto;
  }
  fieldset {
    border: 1px solid #d6d8dc;
    border-radius: 8px;
    margin: 0.75rem 0;
    padding: 0.75rem 1rem;
  }
  label {
    display: block;
    margin: 0.5rem 0;
    font-size: 0.9rem;
  }
  label.row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  input,
  select {
    display: block;
    width: 100%;
    box-sizing: border-box;
    margin-top: 0.25rem;
    padding: 0.4rem 0.6rem;
    border: 1px solid #c8ccd1;
    border-radius: 6px;
  }
  input[type='checkbox'],
  input[type='radio'] {
    width: auto;
    margin: 0;
  }
  .name-cell {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .dot {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .dot-online {
    background: #10b981;
  }
  .dot-offline {
    background: #f87171;
  }
  .dot-never {
    background: #d1d5db;
  }
  details summary {
    cursor: pointer;
    color: #4b5563;
    margin: 0.5rem 0;
  }
  .actions {
    margin-top: 1rem;
    display: flex;
    gap: 0.5rem;
    justify-content: flex-end;
  }
</style>
