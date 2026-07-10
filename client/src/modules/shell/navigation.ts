import {
  Archive,
  Cloud,
  Download,
  History,
  Home,
  MessageSquare,
  PackagePlus,
  Search,
  Settings,
  ShieldCheck,
  type LucideIcon,
} from 'lucide-react';
import type { User } from '../../shared/types';

export type TabKey = 'home' | 'search' | 'sources' | 'profile' | 'history' | 'settings' | 'admin' | 'chat';
export type NavItem = { key: TabKey; labelKey: string; icon: LucideIcon };

const serverBaseTabs: NavItem[] = [
  { key: 'home', labelKey: 'nav.store', icon: Home },
  { key: 'search', labelKey: 'nav.discover', icon: Search },
  { key: 'profile', labelKey: 'nav.myApps', icon: PackagePlus },
];

const serverAdminTab: NavItem = { key: 'admin', labelKey: 'nav.admin', icon: ShieldCheck };
const chatTab: NavItem = { key: 'chat', labelKey: 'nav.chat', icon: MessageSquare };

const clientBaseTabs: NavItem[] = [
	{ key: 'search', labelKey: 'nav.install', icon: Download },
	{ key: 'profile', labelKey: 'nav.installed', icon: Archive },
	{ key: 'sources', labelKey: 'nav.sources', icon: Cloud },
	{ key: 'history', labelKey: 'nav.history', icon: History },
  { key: 'settings', labelKey: 'nav.settings', icon: Settings },
];

export function buildNavItems({
  hasAPI,
  user,
  canReview,
  serverChatVisible,
  clientChatVisible,
}: {
  hasAPI: boolean;
  user: User | null;
  canReview: boolean;
  serverChatVisible: boolean;
  clientChatVisible: boolean;
}) {
  if (!hasAPI) {
    return clientChatVisible
      ? [clientBaseTabs[0], clientBaseTabs[1], chatTab, ...clientBaseTabs.slice(2)]
      : clientBaseTabs;
  }

  if (!user) {
    return serverBaseTabs.filter((item) => item.key !== 'profile');
  }

  return [
    serverBaseTabs[0],
    serverBaseTabs[1],
    ...(serverChatVisible ? [chatTab] : []),
    serverBaseTabs[2],
    ...(canReview ? [serverAdminTab] : []),
  ];
}
