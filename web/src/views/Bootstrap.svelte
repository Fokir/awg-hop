<script lang="ts">
  import { api, ApiError } from '../lib/api';

  type Props = { onDone: () => void };
  const { onDone }: Props = $props();

  let password = $state('');
  let endpoint = $state('');
  let listenPort = $state(51820);
  let tunnelSubnet = $state('10.8.0.0/24');
  let serverTunnelIp = $state('10.8.0.1');
  let dnsServers = $state('1.1.1.1');
  let mtu = $state(1420);
  let interfaceName = $state('awg0');
  let jc = $state(4);
  let jmin = $state(50);
  let jmax = $state(1000);

  let importMode = $state<'fresh' | 'wgeasy'>('fresh');
  let wgeasyFile = $state<File | null>(null);

  let busy = $state(false);
  let error = $state<string | null>(null);

  async function submit() {
    busy = true;
    error = null;
    try {
      await api('/api/v1/setup/bootstrap', {
        body: {
          password,
          ingress: {
            listen_port: listenPort,
            host_endpoint: endpoint,
            tunnel_subnet: tunnelSubnet,
            dns_servers: dnsServers,
            mtu,
            interface_name: interfaceName,
            server_tunnel_ip: serverTunnelIp,
            jc,
            jmin,
            jmax,
          },
        },
      });
      // Авто-логин для удобства
      await api('/api/v1/auth/login', { body: { password } });
      if (importMode === 'wgeasy' && wgeasyFile) {
        const fd = new FormData();
        fd.append('file', wgeasyFile);
        await api('/api/v1/setup/wg-easy-import', { formData: fd });
      }
      onDone();
    } catch (e) {
      error = e instanceof ApiError ? `${e.code}: ${e.message}` : String(e);
    } finally {
      busy = false;
    }
  }
</script>

<div class="bootstrap">
  <h1>AWG Hop — первоначальная настройка</h1>
  <p class="hint">Создайте пароль администратора и параметры входного AmneziaWG-сервера.</p>

  <form
    onsubmit={(e) => {
      e.preventDefault();
      submit();
    }}
  >
    <fieldset>
      <legend>Учётная запись</legend>
      <label>
        Пароль администратора (≥ 8 символов)
        <input type="password" bind:value={password} minlength="8" required />
      </label>
    </fieldset>

    <fieldset>
      <legend>Входной AmneziaWG-сервер</legend>
      <div class="grid">
        <label>
          Endpoint (host:port)
          <input bind:value={endpoint} placeholder="vpn.example.com:51820" />
        </label>
        <label>
          UDP-порт
          <input type="number" bind:value={listenPort} min="1" max="65535" />
        </label>
        <label>
          Подсеть туннеля
          <input bind:value={tunnelSubnet} required />
        </label>
        <label>
          IP сервера в туннеле
          <input bind:value={serverTunnelIp} required />
        </label>
        <label>
          DNS клиентов
          <input bind:value={dnsServers} />
        </label>
        <label>
          MTU
          <input type="number" bind:value={mtu} min="1280" max="9000" />
        </label>
        <label>
          Имя интерфейса
          <input bind:value={interfaceName} required />
        </label>
      </div>
    </fieldset>

    <fieldset>
      <legend>Параметры обфускации AmneziaWG</legend>
      <p class="hint">
        Jc / Jmin / Jmax — количество и размеры джанк-пакетов; должны совпадать у клиента и сервера.
      </p>
      <div class="grid">
        <label>Jc <input type="number" bind:value={jc} min="0" max="32" /></label>
        <label>Jmin <input type="number" bind:value={jmin} min="0" /></label>
        <label>Jmax <input type="number" bind:value={jmax} min="0" /></label>
      </div>
    </fieldset>

    <fieldset>
      <legend>Импорт из wg-easy</legend>
      <label class="row">
        <input type="radio" bind:group={importMode} value="fresh" /> Пропустить (новая установка)
      </label>
      <label class="row">
        <input type="radio" bind:group={importMode} value="wgeasy" /> Импортировать <code>wg0.json</code> wg-easy с AmneziaWG
      </label>
      {#if importMode === 'wgeasy'}
        <input
          type="file"
          accept="application/json"
          onchange={(e) => {
            const t = e.currentTarget as HTMLInputElement;
            wgeasyFile = t.files && t.files[0] ? t.files[0] : null;
          }}
        />
        <p class="hint">
          Поддерживаются wg-easy v15+ с включённым AmneziaWG. Vanilla-WireGuard экспорт будет отклонён.
        </p>
      {/if}
    </fieldset>

    {#if error}<p class="err">{error}</p>{/if}
    <button type="submit" disabled={busy}>{busy ? 'Создаётся…' : 'Создать и войти'}</button>
  </form>
</div>

<style>
  .bootstrap {
    max-width: 720px;
  }
  fieldset {
    border: 1px solid #d6d8dc;
    border-radius: 8px;
    margin-bottom: 1rem;
    padding: 1rem 1.25rem;
  }
  legend {
    padding: 0 0.4rem;
    font-weight: 600;
  }
  label {
    display: block;
    margin: 0.5rem 0;
    font-size: 0.9rem;
    color: #1f2328;
  }
  label.row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  input {
    display: block;
    margin-top: 0.25rem;
    padding: 0.45rem 0.6rem;
    border: 1px solid #c8ccd1;
    border-radius: 6px;
    width: 100%;
    box-sizing: border-box;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 0.5rem 1rem;
  }
  .hint {
    color: #525860;
    font-size: 0.85rem;
    margin: 0.25rem 0 0.75rem;
  }
  .err {
    color: #b00020;
    margin: 0.5rem 0;
  }
  button {
    padding: 0.6rem 1.2rem;
    border: 0;
    border-radius: 6px;
    background: #2563eb;
    color: #fff;
    cursor: pointer;
    font-weight: 600;
  }
  button[disabled] {
    opacity: 0.6;
    cursor: progress;
  }
</style>
