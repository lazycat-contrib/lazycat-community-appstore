export const SOURCE_STALE_MS = 24 * 60 * 60 * 1000;
export const ANNOUNCEMENT_DISMISS_STORAGE_KEY = 'lazycat-appstore-dismissed-announcement';
export const ANNOUNCEMENT_NOTIFY_STORAGE_KEY = 'lazycat-appstore-notified-announcement';
export const THEME_STORAGE_KEY = 'lazycat.theme';

export const RECOMMENDED_DOWNLOAD_MIRRORS = [
  ['美国 1', 'https://gh.h233.eu.org/https://github.com'],
  ['美国 2', 'https://rapidgit.jjda.de5.net/https://github.com'],
  ['美国 3', 'https://gh.ddlc.top/https://github.com'],
  ['美国 4', 'https://gh-proxy.org/https://github.com'],
  ['Fastly', 'https://cdn.gh-proxy.org/https://github.com'],
  ['EdgeOne', 'https://edgeone.gh-proxy.org/https://github.com'],
  ['洛杉矶 1', 'https://ghproxy.it/https://github.com'],
  ['洛杉矶 2', 'https://gh.zwy.one/https://github.com'],
  ['英国伦敦', 'https://ghproxy.net/https://github.com'],
  ['全球 CDN 1', 'https://ghfast.top/https://github.com'],
  ['全球 CDN 2', 'https://wget.la/https://github.com'],
] as const;

export const RECOMMENDED_RAW_MIRRORS = [
  ['GitHub 原生', 'https://raw.githubusercontent.com'],
  ['香港 1', 'https://wget.la/https://raw.githubusercontent.com'],
  ['香港 2', 'https://hk.gh-proxy.org/https://raw.githubusercontent.com'],
  ['香港 3', 'https://hub.glowp.xyz/https://raw.githubusercontent.com'],
  ['韩国 1', 'https://ghfast.top/https://raw.githubusercontent.com'],
  ['韩国 2', 'https://gh.catmak.name/https://raw.githubusercontent.com'],
  ['日本 1', 'https://fastly.jsdelivr.net/gh'],
  ['日本 2', 'https://cdn.gh-proxy.org/https://raw.githubusercontent.com'],
  ['日本 3', 'https://g.blfrp.cn/https://raw.githubusercontent.com'],
] as const;

export function mirrorPresetText(items: readonly (readonly [string, string])[]) {
  return items.map(([name, url]) => `${name}=>${url}`).join('\n');
}
