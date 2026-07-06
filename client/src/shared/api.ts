import i18n from '../i18n';
import { API_BASE, HAS_API } from '../config';

export const CLIENT_API_BASE = '/api/client/v1';

export async function readResponseJSON(response: Response): Promise<any> {
  const text = await response.text();
  if (!text.trim()) return {};
  try {
    return JSON.parse(text);
  } catch {
    if (!response.ok) return {};
    throw new Error(i18n.t('toast.invalidApiResponse'));
  }
}

export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  if (!HAS_API) {
    throw new Error(i18n.t('toast.apiMissing'));
  }
  const response = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    headers: options.body instanceof FormData ? options.headers : { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  });
  const data = await readResponseJSON(response);
  if (!response.ok) {
    throw new Error(data?.error?.message || `HTTP ${response.status}`);
  }
  return data as T;
}

export async function clientApi<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`${CLIENT_API_BASE}${path}`, {
    credentials: 'include',
    headers: options.body instanceof FormData ? options.headers : { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  });
  const data = await readResponseJSON(response);
  if (!response.ok) {
    throw new Error(data?.error?.message || `HTTP ${response.status}`);
  }
  return data as T;
}
