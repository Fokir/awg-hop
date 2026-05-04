<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import { formatBytes, formatHandshake, isFreshHandshake } from '../lib/format';
  import type { UpstreamTunnel } from '../lib/types';

  let upstreams = $state<UpstreamTunnel[]>([]);
  let error = $state<string | null>(null);

  let showForm = $state(false);
  let editing = $state<UpstreamTunnel | null>(null);

  let formName = $state('');
  let formInterface = $state('awg1');
  let formConfig = $state('');
  let formEnabled = $state(true);

  onMount(refresh);

  async function refresh() {
    try {
      upstreams = await api<UpstreamTunnel[]>('/api/v1/upstreams');
      error = null;
    } catch (e) {
      error = e instanceof ApiError ? e.message : String(e);
    }
  }

  function openCreate() {
    editing = null;
    formName = '';
    formInterface = 'awg1';
    formConfig =
      '[Interface]\nPrivateKey = …\nAddress = 10.0.0.2/32\nDNS = 1.1.1.1\nMTU = 1420\n# AmneziaWG params (Jc/Jmin/Jmax/S1..S4/H1..H4) если задаются на удалённом сервере\n\n[Peer]\nPublicKey = …\nEndpoint = remote.example.com:51820\nAllowedIPs = 0.0.0.0/0\nPersistentKeepalive = 25\n';
    formEnabled = true;
    showForm = true;
  }

  function openEdit(t: UpstreamTunnel) {
    editing = t;
    formName = t.name;
    formInterface = t.interface_name;
    formConfig = t.config_text;
    formEnabled = t.enabled;
    showForm = true;
  }

  async function submit() {
    error = null;
    try {
      const body = {
        name: formName,
        interface_name: formInterface,
        config_text: formConfig,
        enabled: formEnabled,
      };
      if (editing) {
        await api(`/api/v1/upstreams/${editing.id}`, { method: 'PATCH', body });
      } else {
        await api('/api/v1/upstreams', { body });
      }
      showForm = false;
      await refresh();
    } catch (e) {
      error = e instanceof ApiError ? `${e.code}: ${e.message}` : String(e);
    }
  }

  async function remove(t: UpstreamTunnel) {
    if (!window.confirm(`Удалить upstream ${t.name}?`)) return;
    try {
      await api(`/api/v1/upstreams/${t.id}`, { method: 'DELETE' });
      await refresh();
    } catch (e) {
      error = e instanceof ApiError ? `${e.code}: ${e.message}` : String(e);
    }
  }
</script>

<section>
  <header class="row">
    <div>
      <h2>Исходящие подключения (upstream)</h2>
      <p class="hint">
        Наш сервер выступает <strong>AWG-клиентом</strong> для удалённого AWG-сервера.
        Клиенты с типом <em>«через upstream»</em> получают выход в интернет через выбранный канал.
      </p>
    </div>
    <button type="button" onclick={openCreate}>+ Новый upstream</button>
  </header>
  {#if error}<p class="err">{error}</p>{/if}

  <table>
    <thead>
      <tr>
        <th>Имя</th>
        <th>Интерфейс</th>
        <th>Включён</th>
        <th>Handshake</th>
        <th>RX / TX</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
      {#each upstreams as t (t.id)}
        <tr class:disabled={!t.enabled}>
          <td>{t.name}</td>
          <td><code>{t.interface_name}</code></td>
          <td>{t.enabled ? 'да' : 'нет'}</td>
          <td>
            {#if t.status?.latest_handshake_unix}
              <span class:fresh={isFreshHandshake(t.status.latest_handshake_unix)}>
                {formatHandshake(t.status.latest_handshake_unix)}
              </span>
            {:else if t.status?.last_error}
              <span class="err" title={t.status.last_error}>ошибка</span>
            {:else}
              <span class="muted">—</span>
            {/if}
          </td>
          <td>
            {#if t.status}
              <small>{formatBytes(t.status.transfer_rx_bytes)} / {formatBytes(t.status.transfer_tx_bytes)}</small>
            {:else}
              <span class="muted">—</span>
            {/if}
          </td>
          <td class="actions">
            <button type="button" class="link" onclick={() => openEdit(t)}>править</button>
            <button type="button" class="link danger" onclick={() => remove(t)}>удалить</button>
          </td>
        </tr>
      {/each}
      {#if upstreams.length === 0}
        <tr><td colspan="6" class="muted center">Upstream-подключений пока нет.</td></tr>
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
      <h3>{editing ? `Upstream «${editing.name}»` : 'Новый upstream'}</h3>
      <form
        onsubmit={(e) => {
          e.preventDefault();
          submit();
        }}
      >
        <label>Имя<input bind:value={formName} required /></label>
        <label>Имя интерфейса (Linux)<input bind:value={formInterface} required /></label>
        <label class="row">
          <input type="checkbox" bind:checked={formEnabled} /> Включён
        </label>
        <label>
          Конфиг <code>.conf</code> для <code>awg-quick</code>
          <textarea bind:value={formConfig} rows="14" spellcheck="false"></textarea>
        </label>
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
    margin-bottom: 1.5rem;
  }
  header.row {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 1rem;
  }
  header h2 {
    margin: 0 0 0.25rem 0;
  }
  header .hint {
    margin: 0;
    color: #6b7280;
    font-size: 0.85rem;
    max-width: 60ch;
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
  tr.disabled {
    opacity: 0.5;
  }
  th {
    text-transform: uppercase;
    font-size: 0.78rem;
    color: #6b7280;
  }
  td.actions button {
    margin-right: 0.5rem;
  }
  .muted {
    color: #9ca3af;
  }
  .fresh {
    color: #047857;
  }
  .err {
    color: #b00020;
  }
  .center {
    text-align: center;
    padding: 1.5rem;
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
    padding: 0;
    color: #1d4ed8;
    cursor: pointer;
  }
  button.link.danger {
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
    width: 640px;
    max-width: 95vw;
    max-height: 90vh;
    overflow: auto;
  }
  label {
    display: block;
    margin: 0.5rem 0;
  }
  label.row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  input,
  textarea {
    display: block;
    width: 100%;
    box-sizing: border-box;
    margin-top: 0.25rem;
    padding: 0.4rem 0.6rem;
    border: 1px solid #c8ccd1;
    border-radius: 6px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }
  input[type='checkbox'] {
    width: auto;
    margin: 0;
  }
  .actions {
    margin-top: 1rem;
    display: flex;
    gap: 0.5rem;
    justify-content: flex-end;
  }
</style>
