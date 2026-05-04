<script lang="ts">
  import { api, ApiError } from '../lib/api';

  let busy = $state(false);
  let error = $state<string | null>(null);
  let info = $state<string | null>(null);

  async function importBackup(file: File) {
    busy = true;
    error = null;
    info = null;
    try {
      const fd = new FormData();
      fd.append('file', file);
      const res = await api<{ status: string; apply_error?: string }>(
        '/api/v1/backup/import',
        { formData: fd },
      );
      if (res.apply_error) {
        info = `Бэкап импортирован, но Apply вернул ошибку: ${res.apply_error}`;
      } else {
        info = 'Бэкап импортирован. Возможно, потребуется перелогиниться.';
      }
    } catch (e) {
      error = e instanceof ApiError ? `${e.code}: ${e.message}` : String(e);
    } finally {
      busy = false;
    }
  }

  async function importWgEasy(file: File) {
    busy = true;
    error = null;
    info = null;
    try {
      const fd = new FormData();
      fd.append('file', file);
      const res = await api<{ status: string; peers: number }>(
        '/api/v1/setup/wg-easy-import',
        { formData: fd },
      );
      info = `wg-easy импортирован: ${res.peers} пиров. Нажмите «Применить».`;
    } catch (e) {
      error = e instanceof ApiError ? `${e.code}: ${e.message}` : String(e);
    } finally {
      busy = false;
    }
  }
</script>

<section>
  <h2>Резервное копирование</h2>

  <h3>Экспорт</h3>
  <p class="hint">Скачает <code>awghop-backup.zip</code> с БД (ключи, пиры, туннели, сессии) и манифестом.</p>
  <p><a class="btn" href="/api/v1/backup/export">Скачать бэкап</a></p>

  <h3>Импорт</h3>
  <p class="hint">
    Загрузите ранее сохранённый <code>awghop-backup.zip</code>. Текущая БД будет переименована в
    <code>awghop.db.bak</code>, после чего применится policy routing.
  </p>
  <input
    type="file"
    accept="application/zip,.zip"
    disabled={busy}
    onchange={(e) => {
      const t = e.currentTarget as HTMLInputElement;
      const f = t.files?.[0];
      if (f) importBackup(f);
      t.value = '';
    }}
  />
</section>

<section>
  <h2>Импорт из wg-easy</h2>
  <p class="hint">
    Поддерживаются wg-easy v15+ с включённым AmneziaWG. Vanilla-WireGuard (без AmneziaWG-блока) будет отклонён согласно §5.5.
  </p>
  <input
    type="file"
    accept="application/json,.json"
    disabled={busy}
    onchange={(e) => {
      const t = e.currentTarget as HTMLInputElement;
      const f = t.files?.[0];
      if (f) importWgEasy(f);
      t.value = '';
    }}
  />
</section>

{#if info}<p class="ok">{info}</p>{/if}
{#if error}<p class="err">{error}</p>{/if}

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
  h3 {
    margin: 1rem 0 0.4rem;
  }
  .hint {
    color: #525860;
    font-size: 0.85rem;
  }
  .ok {
    color: #047857;
  }
  .err {
    color: #b00020;
  }
  .btn {
    display: inline-block;
    padding: 0.45rem 0.85rem;
    background: #2563eb;
    color: #fff;
    border-radius: 6px;
    text-decoration: none;
  }
</style>
