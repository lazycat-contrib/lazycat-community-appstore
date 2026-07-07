import i18n from '../i18n';
import { API_BASE, HAS_API } from '../config';
import type { PaginatedResponse } from './types';

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

export function apiWithUploadProgress<T>(
  path: string,
  options: RequestInit & { onUploadProgress?: (percent: number) => void } = {},
): Promise<T> {
  if (!HAS_API) {
    return Promise.reject(new Error(i18n.t('toast.apiMissing')));
  }
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open(options.method || 'GET', `${API_BASE}${path}`);
    xhr.withCredentials = true;

    const headers = new Headers(options.body instanceof FormData ? options.headers : { 'Content-Type': 'application/json', ...options.headers });
    headers.forEach((value, key) => xhr.setRequestHeader(key, value));

    xhr.upload.onprogress = (event) => {
      if (!event.lengthComputable || !options.onUploadProgress) return;
      options.onUploadProgress(Math.round((event.loaded / event.total) * 100));
    };
    xhr.onload = () => {
      const text = xhr.responseText || '';
      let data: any = {};
      if (text.trim()) {
        try {
          data = JSON.parse(text);
        } catch {
          if (xhr.status >= 200 && xhr.status < 300) {
            reject(new Error(i18n.t('toast.invalidApiResponse')));
            return;
          }
        }
      }
      if (xhr.status < 200 || xhr.status >= 300) {
        reject(new Error(data?.error?.message || `HTTP ${xhr.status}`));
        return;
      }
      resolve(data as T);
    };
    xhr.onerror = () => reject(new Error(i18n.t('toast.apiMissing')));
    xhr.send((options.body as XMLHttpRequestBodyInit | null) || null);
  });
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

export async function fetchAllPaginated<TItem, TKey extends string>(
  request: <T>(path: string) => Promise<T>,
  path: string,
  key: TKey,
  pageSize = 100,
): Promise<TItem[]> {
  const items: TItem[] = [];
  let page = 1;
  for (;;) {
    const separator = path.includes('?') ? '&' : '?';
    const data = await request<PaginatedResponse<TItem, TKey>>(`${path}${separator}page=${page}&pageSize=${pageSize}`);
    items.push(...(data[key] || []));
    if (!data.pagination || page >= data.pagination.totalPages || data[key].length === 0) {
      break;
    }
    page += 1;
  }
  return items;
}
