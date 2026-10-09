// Keep public source URLs intact; repository links are derived only for known hosts.
export function publicSourceURL(value?: string) {
  const raw = value?.trim() || '';
  try {
    const url = new URL(raw);
    return ['https:', 'http:'].includes(url.protocol) && !url.username && !url.password ? raw : '';
  } catch {
    return '';
  }
}

export function repositoryURL(value?: string) {
  const safe = publicSourceURL(value);
  if (!safe) return '';
  const url = new URL(safe);
  const host = url.hostname.toLowerCase();
  const parts = url.pathname.split('/').filter(Boolean);
  if (host === 'gitlab.com') {
    const marker = parts.indexOf('-');
    const repository = marker >= 0 ? parts.slice(0, marker) : parts;
    return repository.length >= 2 ? `https://${host}/${repository.join('/').replace(/\.git$/, '')}` : '';
  }
  if (!['github.com', 'raw.githubusercontent.com', 'gitee.com', 'codeberg.org', 'bitbucket.org'].includes(host) || parts.length < 2) return '';
  return `https://${host === 'raw.githubusercontent.com' ? 'github.com' : host}/${parts[0]}/${parts[1].replace(/\.git$/, '')}`;
}

export function sourceLinks(app: { homepage?: string; installProtected?: boolean; latestVersion?: { downloadUrl?: string; upstreamDownloadUrl?: string; sourceType?: string } }) {
  const version = app.latestVersion;
  const upstream = publicSourceURL(version?.upstreamDownloadUrl);
  const direct = publicSourceURL(version?.downloadUrl);
  const lpkRepository = app.installProtected ? '' : repositoryURL(upstream || direct);
  return { softwareRepository: repositoryURL(app.homepage), lpkRepository };
}
