import type { TabKey } from './navigation';

export type ShellRoute = {
  tab: TabKey;
  appId?: number;
  sourceId?: string;
  mode: 'detail' | 'manage';
};

const tabs: TabKey[] = ['home', 'search', 'wishwall', 'sources', 'profile', 'history', 'settings', 'admin', 'chat'];

export function readShellRoute(value: string, hasAPI: boolean): ShellRoute {
  const url = new URL(value, 'http://store.local');
  const fallback = hasAPI ? 'home' : 'search';
  const legacyTab = url.pathname.slice(1) as TabKey;
  const view = url.searchParams.get('view') as TabKey | null;
  const route: ShellRoute = {
    tab: tabs.includes(legacyTab) ? legacyTab : fallback,
    mode: 'detail',
  };
  if (url.pathname !== '/' && !tabs.includes(legacyTab)) return route;
  if (view && tabs.includes(view)) route.tab = view;
  const rawID = url.searchParams.get('app') || '';
  const id = Number(rawID);
  if (/^[1-9]\d*$/.test(rawID) && Number.isSafeInteger(id)) {
    route.appId = id;
    route.sourceId = url.searchParams.get('source') || undefined;
    route.mode = hasAPI && url.searchParams.get('mode') === 'manage' ? 'manage' : 'detail';
  }
  return route;
}

export function shellRouteURL(route: { tab: TabKey; appId?: number; sourceId?: string | number; mode?: 'detail' | 'manage' }) {
  const params = new URLSearchParams({ view: route.tab });
  if (route.appId) {
    params.set('app', String(route.appId));
    if (route.sourceId !== undefined) params.set('source', String(route.sourceId));
    if (route.mode === 'manage') params.set('mode', route.mode);
  }
  return `/?${params}`;
}

export function authenticationDestination(returnTo: string, next: string | null, canReview: boolean) {
  if (next === 'submit') return shellRouteURL({ tab: 'profile' });
  if (next === 'admin' && canReview) return shellRouteURL({ tab: 'admin' });
  return returnTo;
}

export function needsClientCatalog(tab: TabKey) {
  return ['sources', 'search', 'history', 'profile'].includes(tab);
}
