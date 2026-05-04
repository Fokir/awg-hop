export function formatBytes(n: number): string {
  if (!n || n <= 0) return '0';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return v.toFixed(v >= 100 ? 0 : v >= 10 ? 1 : 2) + ' ' + units[i];
}

export function formatHandshake(unixSec: number): string {
  if (!unixSec) return '—';
  const ageSec = Math.max(0, Date.now() / 1000 - unixSec);
  if (ageSec < 60) return `${Math.floor(ageSec)} с назад`;
  if (ageSec < 3600) return `${Math.floor(ageSec / 60)} мин назад`;
  if (ageSec < 86400) return `${Math.floor(ageSec / 3600)} ч назад`;
  return `${Math.floor(ageSec / 86400)} дн назад`;
}

export function isFreshHandshake(unixSec: number): boolean {
  if (!unixSec) return false;
  return Date.now() / 1000 - unixSec < 180;
}
