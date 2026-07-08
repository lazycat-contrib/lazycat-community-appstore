import { API_BASE, HAS_API } from '../../../config';
import { ApiRequestError, readResponseJSON } from '../../../shared/api';
import type { ApiClient, MigrationImportInput, MigrationImportResult, MigrationModuleOptions, MigrationPreview } from './types';

export async function exportMigrationPackage(_api: ApiClient, options: MigrationModuleOptions): Promise<void> {
  if (!HAS_API) throw new Error('API is not available');
  const response = await fetch(`${API_BASE}/api/v1/admin/migration/export`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(options),
  });
  if (!response.ok) {
    const data = await readResponseJSON(response);
    throw new ApiRequestError(data?.error?.message || `HTTP ${response.status}`, response.status, data?.error?.code, data?.error?.details);
  }
  const blob = await response.blob();
  const filename = filenameFromDisposition(response.headers.get('Content-Disposition')) || 'lazycat-appstore-migration.zip';
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  anchor.rel = 'noopener';
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

export async function previewMigrationPackage(api: ApiClient, file: File): Promise<MigrationPreview> {
  const body = new FormData();
  body.set('file', file);
  const data = await api<{ preview: MigrationPreview }>('/api/v1/admin/migration/import/preview', { method: 'POST', body });
  return data.preview;
}

export async function importMigrationPackage(api: ApiClient, input: MigrationImportInput): Promise<MigrationImportResult> {
  const body = new FormData();
  body.set('file', input.file);
  body.set('mode', input.mode);
  body.set('includeSite', String(input.options.includeSite));
  body.set('includePeople', String(input.options.includePeople));
  body.set('includeApps', String(input.options.includeApps));
  body.set('includeFiles', String(input.options.includeFiles));
  if (input.confirmReplace) body.set('confirmReplace', input.confirmReplace);
  const data = await api<{ result: MigrationImportResult }>('/api/v1/admin/migration/import', { method: 'POST', body });
  return data.result;
}

function filenameFromDisposition(value: string | null) {
  if (!value) return '';
  const match = value.match(/filename="([^"]+)"/i) || value.match(/filename=([^;]+)/i);
  return match?.[1]?.trim() || '';
}
