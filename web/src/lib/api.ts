function readCookie(name: string): string {
  const match = document.cookie.split(';').find((c) => c.trim().startsWith(name + '='));
  return match ? decodeURIComponent(match.trim().slice(name.length + 1)) : '';
}

export class ApiError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export interface ApiOptions {
  method?: string;
  body?: unknown;
  raw?: boolean;
  responseType?: 'json' | 'blob' | 'text';
  signal?: AbortSignal;
  formData?: FormData;
}

export async function api<T = unknown>(path: string, opts: ApiOptions = {}): Promise<T> {
  const headers: Record<string, string> = {};
  let body: BodyInit | undefined;

  if (opts.formData) {
    body = opts.formData;
  } else if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json';
    body = JSON.stringify(opts.body);
  }

  const method = opts.method ?? (body ? 'POST' : 'GET');
  if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
    const token = readCookie('awghop_csrf');
    if (token) headers['X-CSRF-Token'] = token;
  }

  const res = await fetch(path, {
    method,
    credentials: 'include',
    headers,
    body,
    signal: opts.signal,
  });

  if (!res.ok) {
    let code = 'http_' + res.status;
    let message = res.statusText;
    try {
      const data = await res.json();
      if (data && typeof data === 'object') {
        if (typeof data.error === 'string') code = data.error;
        if (typeof data.message === 'string') message = data.message;
      }
    } catch {
      // ignore
    }
    throw new ApiError(res.status, code, message);
  }

  if (opts.responseType === 'blob') return (await res.blob()) as T;
  if (opts.responseType === 'text') return (await res.text()) as T;
  if (res.status === 204) return undefined as T;

  const ct = res.headers.get('Content-Type') ?? '';
  if (ct.includes('application/json')) return (await res.json()) as T;
  return (await res.text()) as T;
}

export const url = (p: string) => p;
