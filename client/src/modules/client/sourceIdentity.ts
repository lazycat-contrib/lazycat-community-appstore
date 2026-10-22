type SourceIdentity = { id: string | number; url: string };

function parseSourceURL(value: string): URL | undefined {
  try {
    const url = new URL(value.trim());
    return url.protocol === 'http:' || url.protocol === 'https:' ? url : undefined;
  } catch {
    return undefined;
  }
}

export function normalizeSourceIdentity(value: string): string {
  const url = parseSourceURL(value);
  if (!url) return '';
  url.username = '';
  url.password = '';
  url.hash = '';
  url.pathname = url.pathname.replace(/\/+$/, '') || '/';
  url.searchParams.sort();
  return url.href;
}

export function sourceDisplayURL(value: string): string {
  const url = parseSourceURL(value);
  if (!url) return '';
  return `${url.origin}${url.pathname}`;
}

export function sourceColor(value: string): string | undefined {
  const identity = normalizeSourceIdentity(value);
  if (!identity) return undefined;
  let hash = 2166136261;
  for (const character of identity) {
    hash = Math.imul(hash ^ character.charCodeAt(0), 16777619);
  }
  return `hsl(${(hash >>> 0) % 360} 48% 44%)`;
}

export function sourceColorMap(sources: readonly SourceIdentity[]): Map<string, string> {
  const colors = new Map<string, string>();
  for (const source of sources) {
    const color = sourceColor(source.url);
    if (color) colors.set(String(source.id), color);
  }
  return colors;
}
