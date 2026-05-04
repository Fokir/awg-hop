<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import type { IngressSettings, SystemSettings } from '../lib/types';

  let ingress = $state<IngressSettings | null>(null);
  let system = $state<SystemSettings | null>(null);
  let error = $state<string | null>(null);
  let saved = $state<string | null>(null);

  onMount(refresh);

  async function refresh() {
    try {
      [ingress, system] = await Promise.all([
        api<IngressSettings>('/api/v1/settings/ingress'),
        api<SystemSettings>('/api/v1/settings/system'),
      ]);
      error = null;
    } catch (e) {
      error = e instanceof ApiError ? e.message : String(e);
    }
  }

  async function saveIngress() {
    if (!ingress) return;
    saved = null;
    error = null;
    try {
      ingress = await api<IngressSettings>('/api/v1/settings/ingress', {
        method: 'PUT',
        body: ingress,
      });
      saved = 'Настройки входного сервера сохранены. Не забудьте «Применить» для перевыкатывания интерфейса.';
    } catch (e) {
      error = e instanceof ApiError ? `${e.code}: ${e.message}` : String(e);
    }
  }

  async function saveSystem() {
    if (!system) return;
    saved = null;
    error = null;
    try {
      system = await api<SystemSettings>('/api/v1/settings/system', {
        method: 'PUT',
        body: system,
      });
      saved = 'Системные настройки сохранены.';
    } catch (e) {
      error = e instanceof ApiError ? `${e.code}: ${e.message}` : String(e);
    }
  }
</script>

{#if error}<p class="err">{error}</p>{/if}
{#if saved}<p class="ok">{saved}</p>{/if}

{#if ingress}
  <section>
    <h2>Входной AmneziaWG</h2>
    <form
      onsubmit={(e) => {
        e.preventDefault();
        saveIngress();
      }}
    >
      <div class="grid">
        <label>Endpoint (host:port)<input bind:value={ingress.host_endpoint} placeholder="vpn.example.com:51820" /></label>
        <label>UDP-порт<input type="number" bind:value={ingress.listen_port} min="1" max="65535" /></label>
        <label>Имя интерфейса<input bind:value={ingress.interface_name} required /></label>
        <label>Подсеть туннеля<input bind:value={ingress.tunnel_subnet} required /></label>
        <label>IP сервера в туннеле<input bind:value={ingress.server_tunnel_ip} required /></label>
        <label>DNS клиентов<input bind:value={ingress.dns_servers} /></label>
        <label>MTU<input type="number" bind:value={ingress.mtu} min="1280" max="9000" /></label>
        <label class="full">Server PublicKey<input bind:value={ingress.server_public_key} readonly /></label>
      </div>

      <h3>Параметры обфускации AmneziaWG</h3>
      <p class="hint">Эти значения копируются в клиентский <code>.conf</code> и должны совпадать у клиента и сервера.</p>
      <div class="grid">
        <label>Jc<input type="number" bind:value={ingress.jc} min="0" max="32" /></label>
        <label>Jmin<input type="number" bind:value={ingress.jmin} min="0" /></label>
        <label>Jmax<input type="number" bind:value={ingress.jmax} min="0" /></label>
        <label>S1<input bind:value={ingress.s1} /></label>
        <label>S2<input bind:value={ingress.s2} /></label>
        <label>S3<input bind:value={ingress.s3} /></label>
        <label>S4<input bind:value={ingress.s4} /></label>
        <label>H1<input type="number" bind:value={ingress.h1} /></label>
        <label>H2<input type="number" bind:value={ingress.h2} /></label>
        <label>H3<input type="number" bind:value={ingress.h3} /></label>
        <label>H4<input type="number" bind:value={ingress.h4} /></label>
      </div>

      <button type="submit">Сохранить входные настройки</button>
    </form>
  </section>
{/if}

{#if system}
  <section>
    <h2>Системные настройки</h2>
    <form
      onsubmit={(e) => {
        e.preventDefault();
        saveSystem();
      }}
    >
      <div class="grid">
        <label>
          Внешний интерфейс (для NAT direct-пиров; пусто = автоопределение)
          <input bind:value={system.external_interface} placeholder="eth0" />
        </label>
        <label>
          Политика недоступного туннеля
          <select bind:value={system.tunnel_offline_policy}>
            <option value="block">block — отказать в Apply</option>
            <option value="ignore">ignore — пропустить пира</option>
          </select>
        </label>
        <label>
          AllowedIPs клиента (IPv4)
          <input bind:value={system.client_allowed_ipv4} placeholder="0.0.0.0/0" />
        </label>
        <label>
          AllowedIPs клиента (IPv6, пусто = выключено)
          <input bind:value={system.client_allowed_ipv6} placeholder="::/0" />
        </label>
      </div>

      <button type="submit">Сохранить системные настройки</button>
    </form>
  </section>
{/if}

<style>
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
  .hint {
    color: #525860;
    font-size: 0.85rem;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 0.5rem 1rem;
  }
  label {
    display: block;
    margin: 0.25rem 0;
    font-size: 0.9rem;
  }
  label.full {
    grid-column: 1/-1;
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
  button {
    margin-top: 1rem;
    padding: 0.55rem 1rem;
    border: 0;
    border-radius: 6px;
    background: #2563eb;
    color: #fff;
    cursor: pointer;
    font-weight: 600;
  }
  .err {
    color: #b00020;
  }
  .ok {
    color: #047857;
  }
</style>
