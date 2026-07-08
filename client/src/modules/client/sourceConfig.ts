export type ParsedSourceConfig = {
  kind: 'url' | 'group-code' | 'config';
  url: string;
  groupCodes: string[];
};

export function normalizeGroupCode(value: string): string {
  const code = value.trim().toUpperCase();
  return /^[A-Z0-9]{6}$/.test(code) ? code : '';
}

export function normalizeGroupCodes(values: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  values.forEach((value) => {
    const code = normalizeGroupCode(value);
    if (!code || seen.has(code)) return;
    seen.add(code);
    out.push(code);
  });
  return out;
}

export function parseSourceConfigInput(raw: string, defaultSourceUrl: string): ParsedSourceConfig | null {
  const value = raw.trim();
  if (!value) return null;

  const decoded = decodeBase64JSON(value);
  if (decoded) {
    const url = normalizeSourceURL(String(decoded.sourceUrl || decoded.url || defaultSourceUrl));
    const groupCodes = normalizeGroupCodes([
      ...arrayOfStrings(decoded.groupCodes),
      ...arrayOfStrings(decoded.codes),
      ...arrayOfGroupCodes(decoded.groups),
    ]);
    if (url) return { kind: 'config', url, groupCodes };
  }

  const singleCode = normalizeGroupCode(value);
  if (singleCode) {
    const url = normalizeSourceURL(defaultSourceUrl);
    return url ? { kind: 'group-code', url, groupCodes: [singleCode] } : null;
  }

  const url = normalizeSourceURL(value);
  return url ? { kind: 'url', url, groupCodes: [] } : null;
}

export function normalizeSourceURL(rawURL: string): string {
  try {
    const parsed = new URL(rawURL.trim());
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return '';
    parsed.hash = '';
    return parsed.toString().replace(/\/$/, '');
  } catch {
    return '';
  }
}

function decodeBase64JSON(value: string): Record<string, unknown> | null {
  try {
    const normalized = value.replace(/\s/g, '');
    if (!/^[A-Za-z0-9+/=_-]+$/.test(normalized)) return null;
    const padded = normalized.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(normalized.length / 4) * 4, '=');
    const decoded = window.atob(padded);
    const parsed = JSON.parse(decoded);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed as Record<string, unknown> : null;
  } catch {
    return null;
  }
}

function arrayOfStrings(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];
}

function arrayOfGroupCodes(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value
    .map((item) => {
      if (!item || typeof item !== 'object') return '';
      const group = item as { code?: unknown };
      return typeof group.code === 'string' ? group.code : '';
    })
    .filter(Boolean);
}
