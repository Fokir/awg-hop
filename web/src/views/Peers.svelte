<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import { formatBytes, formatHandshake, isFreshHandshake } from '../lib/format';
  import type { Peer, EgressTunnel } from '../lib/types';

  let peers = $state<Peer[]>([]);
  let tunnels = $state<EgressTunnel[]>([]);
  let error = $state<string | null>(null);

  let showForm = $state(false);
  let editing = $state<Peer | null>(null);

  let formName = $state('');
  let formAllowedIps = $state('');
  let formPrivateKey = $state('');
  let formPublicKey = $state('');
  let formGeneratePsk = $state(false);
  let formEgressType = $state<'direct' | 'egress_awg'>('direct');
  let formEgressTunnelId = $state<number | ''>('');
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
      [peers, tunnels] = await Promise.all([
        api<Peer[]>('/api/v1/peers'),
        api<EgressTunnel[]>('/api/v1/egress-tunnels'),
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
    formEgressType = 'direct';
    formEgressTunnelId = '';
    formEnabled = true;
    showForm = true;
  }

  function openEdit(p: Peer) {
    editing = p;
    formName = p.name;
    formAllowedIps = p.allowed_ips;
    formPrivateKey = '';
    formPublicKey = p.public_key;
    formGeneratePsk = false;
    formEgressType = p.egress_type;
    formEgressTunnelId = p.egress_tunnel_id ?? '';
    formEnabled = p.enabled;
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
          egress_type: formEgressType,
          egress_tunnel_id: formEgressType === 'egress_awg' ? Number(formEgressTunnelId) : null,
        };
        await api(`/api/v1/peers/${editing.id}`, { method: 'PATCH', body });
      } else {
        const body: Record<string, unknown> = {
          name: formName,
          allowed_ips: formAllowedIps || undefined,
          private_key: formPrivateKey || undefined,
          public_key: formPublicKey || undefined,
          generate_psk: formGeneratePsk,
          egress_type: formEgressType,
          egress_tunnel_id: formEgressType === 'egress_awg' ? Number(formEgressTunnelId) : undefined,
        };
        await api('/api/v1/peers', { body });
      }
      showForm = false;
      await refresh();
    } catch (e) {
      error = e instanceof ApiError ? `${e.code}: ${e.message}` : String(e);
    }
  }

  async function toggle(p: Peer) {
    try {
      await api(`/api/v1/peers/${p.id}/${p.enabled ? 'disable' : 'enable'}`, { method: 'POST' });
      await refresh();
    } catch (e) {
      error = e instanceof ApiError ? e.message : String(e);
    }
  }

  async function remove(p: Peer) {
    if (!window.confirm(`Удалить пира ${p.name}?`)) return;
    try {
      await api(`/api/v1/peers/${p.id}`, { method: 'DELETE' });
      await refresh();
    } catch (e) {
      error = e instanceof ApiError ? e.message : String(e);
    }
  }

  function tunnelName(id: number | null | undefined) {
    if (!id) return '';
    return tunnels.find((t) => t.id === id)?.name ?? `#${id}`;
  }
</script>

<section>
  <header class="row">
    <h2>Пиры входа (AmneziaWG)</h2>
    <button type="button" onclick={openCreate}>+ Новый пир</button>
  </header>
  {#if error}<p class="err">{error}</p>{/if}

  <table>
    <thead>
      <tr>
        <th>Имя</th>
        <th>Адрес</th>
        <th>Egress</th>
        <th>Handshake</th>
        <th>RX / TX</th>
        <th>Статус</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
      {#each peers as p (p.id)}
        <tr class:disabled={!p.enabled}>
          <td>{p.name}</td>
          <td><code>{p.allowed_ips}</code></td>
          <td>
            {#if p.egress_type === 'direct'}
              <span class="badge direct">direct</span>
            {:else}
              <span class="badge tunnel">→ {tunnelName(p.egress_tunnel_id)}</span>
            {/if}
          </td>
          <td>
            {#if p.status?.latest_handshake_unix}
              <span class:fresh={isFreshHandshake(p.status.latest_handshake_unix)}>
                {formatHandshake(p.status.latest_handshake_unix)}
              </span>
            {:else}
              <span class="muted">—</span>
            {/if}
          </td>
          <td>
            {#if p.status}
              <small>{formatBytes(p.status.transfer_rx_bytes)} / {formatBytes(p.status.transfer_tx_bytes)}</small>
            {:else}
              <span class="muted">—</span>
            {/if}
          </td>
          <td>
            <button type="button" class="link" onclick={() => toggle(p)}>
              {p.enabled ? 'выкл' : 'вкл'}
            </button>
          </td>
          <td class="actions">
            <a href={`/api/v1/peers/${p.id}/config`}>conf</a>
            <a href={`/api/v1/peers/${p.id}/qrcode`} target="_blank" rel="noreferrer">QR</a>
            <button type="button" class="link" onclick={() => openEdit(p)}>править</button>
            <button type="button" class="link danger" onclick={() => remove(p)}>удалить</button>
          </td>
        </tr>
      {/each}
      {#if peers.length === 0}
        <tr><td colspan="7" class="muted center">Пиров пока нет.</td></tr>
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
      <h3>{editing ? `Пир «${editing.name}»` : 'Новый пир'}</h3>
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
          <legend>Egress</legend>
          <label class="row">
            <input type="radio" bind:group={formEgressType} value="direct" />
            Прямой выход в интернет (NAT контейнера)
          </label>
          <label class="row">
            <input type="radio" bind:group={formEgressType} value="egress_awg" />
            Через исходящий AWG-туннель
          </label>
          {#if formEgressType === 'egress_awg'}
            <select bind:value={formEgressTunnelId} required>
              <option value="" disabled>— выберите туннель —</option>
              {#each tunnels as t}
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
