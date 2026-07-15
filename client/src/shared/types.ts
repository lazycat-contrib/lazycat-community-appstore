export type User = {
  id: number;
  username: string;
  nickname?: string;
  email?: string;
  role: 'USER' | 'SOFTWARE_ADMIN' | 'SITE_ADMIN';
  emailVerified?: boolean;
  twoFactorEnabled?: boolean;
  avatarUrl?: string;
  disabled?: boolean;
};

export type Pagination = {
  page: number;
  pageSize: number;
  totalItems: number;
  totalPages: number;
};

export type RuntimeCapabilities = {
  lazycatInstall: boolean;
  githubMirrors: GitHubMirrorOption[];
};

export type PaginatedResponse<TItem, TKey extends string = 'items'> = {
  pagination: Pagination;
} & {
  [K in TKey]: TItem[];
};

export type Version = {
  id: number;
  appId: number;
  version: string;
  changelog: string;
  status: string;
  sourceType: string;
  downloadUrl: string;
  fileSize: number;
  sha256: string;
  storageKey?: string;
  createdAt: string;
  publishedAt?: string;
};

export type VersionRetentionPolicy = {
  mode: 'INHERIT' | 'CUSTOM';
  siteMaxVersions: number;
  appMaxVersions?: number | null;
  effectiveMaxVersions: number;
};

export type LPKInspectionStatus = {
  id: number;
  state: 'PENDING' | 'RUNNING' | 'SUCCEEDED' | 'FAILED' | 'TIMED_OUT' | 'CANCELLED' | string;
  trigger: 'API_TOKEN_FIRST_SUBMISSION' | 'MANUAL' | string;
  attempts: number;
  lastError?: string;
  updatedAt: string;
  completedAt?: string;
};

export type VersionCleanupWarning = {
  versionId: number;
  storageKey: string;
  storagePath: string;
  message: string;
};

export type StoreApp = {
  id: number;
  ownerId: number;
  owner: string;
  packageId?: string;
  categoryId?: number;
  name: string;
  nameI18n?: Record<string, string>;
  slug: string;
  summary: string;
  summaryI18n?: Record<string, string>;
  description: string;
  descriptionI18n?: Record<string, string>;
  author?: string;
  homepage?: string;
  license?: string;
  minOSVersion?: string;
  iconUrl?: string;
  status: string;
  category?: string;
  categoryI18n?: Record<string, string>;
  tags: string[];
  visibleGroupIds: number[];
  allowUnreviewedUpdates: boolean;
  commentsEnabled: boolean;
  commentsAllowed?: boolean;
  emailNotificationsEnabled: boolean;
  installProtected: boolean;
  downloadCount: number;
  downloadStats?: DownloadStats;
  rating?: RatingSummary;
  appFavorited?: boolean;
  submitterFavorited?: boolean;
  latestVersion?: Version;
  versions?: Version[];
  versionRetention?: VersionRetentionPolicy;
  lpkInspection?: LPKInspectionStatus;
  screenshots?: Screenshot[];
  comments?: Comment[];
  favorites?: number;
  outdatedMarks?: number;
  outdatedMarked?: boolean;
  canManageApp?: boolean;
  canUploadVersion?: boolean;
  canClearOutdatedMarks?: boolean;
  updatedAt: string;
};

export type DownloadStats = {
  total: number;
  day: number;
  week: number;
  month: number;
  year: number;
};

export type RatingSummary = {
  score: number;
  voteCount: number;
  voted?: boolean;
};

export type Screenshot = {
  id: number;
  appId: number;
  imageUrl: string;
  storageKey?: string;
  caption: string;
  deviceType?: 'DESKTOP' | 'MOBILE' | string;
  sortOrder: number;
};

export type Comment = {
  id: number;
  userId: number;
  parentId?: number;
  authorType?: 'USER' | 'CLIENT' | string;
  clientUserId?: string;
  username: string;
  body: string;
  canDelete?: boolean;
  replies?: Comment[];
  createdAt: string;
};

export type Review = {
  id: number;
  kind: string;
  status: string;
  appId?: number;
  versionId?: number;
  requesterId: number;
  note: string;
  reviewNote?: string;
  createdAt: string;
};

export type Category = {
  id: number;
  name: string;
  nameI18n?: Record<string, string>;
  slug: string;
  parentId?: number;
  sortOrder?: number;
};

export type TagRecord = {
  id: number;
  name: string;
  nameI18n?: Record<string, string>;
  slug: string;
};

export type Collection = {
  id: number;
  name: string;
  slug: string;
  description: string;
  kind: string;
  apps: StoreApp[];
};

export type CollaboratorRequest = {
  id: number;
  app_id?: number;
  appId?: number;
  appName?: string;
  user_id?: number;
  userId?: number;
  username?: string;
  email?: string;
  status: string;
  message: string;
  created_at?: string;
  createdAt?: string;
};

export type Collaborator = {
  id: number;
  appId: number;
  appName: string;
  userId: number;
  username: string;
  email?: string;
  createdAt: string;
};

export type CollaboratorInvite = {
  id: number;
  appId: number;
  appName: string;
  email?: string;
  tokenPrefix: string;
  inviteUrl?: string;
  acceptedBy?: number;
  acceptedAt?: string;
  expiresAt: string;
  createdAt: string;
};

export type OwnedCollaboration = {
  app: StoreApp;
  collaborators: Collaborator[];
  requests: CollaboratorRequest[];
  invites: CollaboratorInvite[];
};

export type CollaborationData = {
  owned: OwnedCollaboration[];
  collaborating: StoreApp[];
  outgoingRequests: CollaboratorRequest[];
};

export type Group = {
  id: number;
  owner_id?: number;
  ownerId?: number;
  name: string;
  slug: string;
  description: string;
  code?: string;
  codeUpdatedAt?: string;
  memberCount?: number;
  attachedAppCount?: number;
};

export type APITokenRecord = {
  id: number;
  name: string;
  prefix: string;
  created_at?: string;
  createdAt?: string;
};

export type MCPPrincipalType = 'USER' | 'ADMIN';

export type MCPProfile = {
  endpoint: string;
  sourceUrl?: string;
  principalTypes: MCPPrincipalType[];
};

export type MCPTokenRecord = {
  id: number;
  note: string;
  prefix: string;
  principalType: MCPPrincipalType;
  expiresAt?: string;
  lastUsedAt?: string;
  createdAt: string;
};

export type RegistrationInvite = {
  id: number;
  code: string;
  codePrefix: string;
  note: string;
  maxUses: number;
  remainingUses: number;
  createdBy: number;
  createdAt: string;
  updatedAt: string;
};

export type StorageOption = {
  key: string;
  name: string;
  isDefault: boolean;
  provider: string;
  deliveryMode: string;
};

export type SourceID = number | string;

export type AdsPreference = 'unset' | 'enabled' | 'disabled';

export type GitHubMirror = {
  id: string;
  kind: 'download' | 'raw';
  name: string;
  url: string;
};

export type GitHubMirrorOption = Pick<GitHubMirror, 'id' | 'kind' | 'name'>;

export type InstallMirrorConfig = {
  githubMirrors: GitHubMirrorOption[];
  defaultDownloadMirrorId: string;
  defaultRawMirrorId: string;
};

export type SourceSubscription = {
  id: SourceID;
  name: string;
  url: string;
  password: string;
  defaultDownloadMirrorId: string;
  defaultRawMirrorId: string;
  groupCodes?: string[];
  groups?: Array<{ name: string; code?: string }>;
  categories?: Category[];
  announcements?: SiteAnnouncement[];
  ads?: SiteAd[];
  clientPolicy?: ClientPolicy;
  lastInvalidGroupCodes?: string[];
  githubMirrors: GitHubMirror[];
  chatAvailable?: boolean;
  chatEnabled?: boolean;
  adsPreference?: AdsPreference;
  lastSync?: string;
  lastError?: string;
  lastErrorCode?: SourceErrorCode;
  lastAppCount?: number;
  lastInstallableCount?: number;
};

export type SourceInput = Pick<
  SourceSubscription,
  'name' | 'url' | 'password' | 'defaultDownloadMirrorId' | 'defaultRawMirrorId' | 'groupCodes' | 'chatEnabled' | 'adsPreference'
>;

export type ClientSettings = {
  clientTitle: string;
  commentDisplayName: string;
  defaultPageSize: number;
  autoSyncEnabled: boolean;
  autoSyncIntervalMinutes: number;
  syncOnStartup: boolean;
  installSuccessDismissSeconds: number;
  lastAutoSyncAt?: string;
  lastAutoSyncStatus?: string;
  lastAutoSyncError?: string;
  autoUpdateEnabled: boolean;
  autoUpdateIntervalMinutes: number;
  lastAutoUpdateAt?: string;
  lastAutoUpdateStatus?: string;
  lastAutoUpdateError?: string;
};

export type ClientIdentity = {
  id: string;
  displayName: string;
  email?: string;
  avatarUrl?: string;
  source: 'header' | 'oidc' | 'local' | string;
};

export type ClientAuthStatus = {
  authenticated: boolean;
  oidcEnabled: boolean;
  user?: ClientIdentity | null;
};

export type CommentNotification = {
  id: number;
  appId: number;
  commentId: number;
  appName: string;
  actorName: string;
  body: string;
  read: boolean;
  createdAt: string;
};

export type SourceVersion = {
  version: string;
  changelog?: string;
  downloadUrl: string;
  upstreamDownloadUrl?: string;
  sourceType?: string;
  sha256: string;
  size: number;
};

export type SourceApp = {
  id: number;
  sourceId?: SourceID;
  sourceName: string;
  externalId?: string;
  packageId?: string;
  name: string;
  nameI18n?: Record<string, string>;
  slug: string;
  summary: string;
  summaryI18n?: Record<string, string>;
  descriptionI18n?: Record<string, string>;
  author?: string;
  homepage?: string;
  license?: string;
  minOSVersion?: string;
  categoryId?: number;
  category?: string;
  categoryI18n?: Record<string, string>;
  iconUrl?: string;
  installProtected?: boolean;
  commentsEnabled?: boolean;
  outdatedMarks?: number;
  screenshots?: Screenshot[];
  latestVersion?: SourceVersion;
  versions?: SourceVersion[];
};

export type FavoriteData = {
  apps: StoreApp[];
  submitters: User[];
};

export type SetupStatus = {
  needsSetup: boolean;
};

export type SiteAnnouncement = {
  id?: number;
  enabled: boolean;
  level: 'info' | 'warning' | 'success';
  title?: string;
  body?: string;
  linkLabel?: string;
  linkUrl?: string;
  startsAt?: string;
  endsAt?: string;
  sortOrder?: number;
  sourceName?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type SiteAd = {
  id?: number;
  enabled: boolean;
  title?: string;
  body?: string;
  imageUrl?: string;
  linkLabel?: string;
  linkUrl?: string;
  startsAt?: string;
  endsAt?: string;
  sortOrder?: number;
  sourceName?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type ClientPolicy = {
  minVersion?: string;
  message?: string;
};

export type RegistrationMode = 'open' | 'invite' | 'closed';

export type SiteRegistration = {
  mode: RegistrationMode;
};

export type SiteChat = {
  enabled: boolean;
  retentionDays: number;
};

export type SiteSecurity = {
  twoFactorAuthEnabled: boolean;
};

export type SiteProfile = {
  title: string;
  subtitle?: string;
  iconUrl?: string;
  publicUrl: string;
  sourceUrl: string;
  timeZone?: string;
  version?: string;
  defaultPageSize?: number;
  announcement: SiteAnnouncement;
  announcements?: SiteAnnouncement[];
  ads?: SiteAd[];
  registration: SiteRegistration;
  clientPolicy?: ClientPolicy;
  chat: SiteChat;
  security: SiteSecurity;
};

export type BackupTargetResult = {
  storageKey: string;
  storageName: string;
  directory?: string;
  objectPath?: string;
  downloadUrl?: string;
  status: 'success' | 'partial' | 'failed' | string;
  error?: string;
};

export type BackupRunResult = {
  startedAt: string;
  finishedAt?: string;
  trigger: 'manual' | 'scheduled' | string;
  status: 'success' | 'partial' | 'failed' | string;
  objectPath?: string;
  size?: number;
  sha256?: string;
  manifestCounts?: Record<string, number>;
  warnings?: string[];
  targets?: BackupTargetResult[];
  error?: string;
};

export type BackupTargetSettings = {
  storageKey: string;
  directory: string;
};

export type BackupSettings = {
  enabled: boolean;
  scheduleTime: string;
  retentionCount: number;
  storageKeys: string[];
  targets?: BackupTargetSettings[];
  lastRun?: BackupRunResult;
  isRunning: boolean;
};

export type Toast = {
  tone: 'success' | 'error' | 'neutral';
  message: string;
};

export type InstallActivity = {
  appKey: string;
  appId: number;
  version: string;
  title: string;
  source: string;
  checksum: string;
  taskId?: string;
  status: 'running' | 'success' | 'error' | 'cancelled';
  progress: number;
  progressKnown?: boolean;
  stageKey: string;
  detail?: string;
  isCancelling?: boolean;
  resultMode?: string;
  messageKey?: string;
  messageParams?: Record<string, string | number>;
};

export type InstallPasswordRequest = {
  app: StoreApp | SourceApp;
  version?: string;
};

export type InstallOptions = {
  installPassword?: string;
  version?: string;
  mirrorId?: string;
  confirmed?: boolean;
};

export type ClientSourceStats = {
  sourceCount: number;
  syncedSourceCount: number;
  staleSourceCount: number;
  authSourceCount: number;
  failedSourceCount: number;
  sourceAppCount: number;
  installableSourceAppCount: number;
};

export type SourceErrorCode = 'auth' | 'format' | 'http' | 'network';

export type InstalledApplication = {
  appid?: string;
  title?: string;
  version?: string;
  status?: string;
  instanceStatus?: string;
  icon?: string;
  autoUpdateEnabled?: boolean;
};

export type ClientInstallTask = {
  taskId: string;
  status: string;
  downloadedSize?: number;
  totalSize?: number;
  detail?: string;
};

export type UpdateQueueItem = {
  appId: number;
  sourceId: number;
  sourceName: string;
  packageId: string;
  appName: string;
  installedVersion?: string;
  version?: string;
  status: 'queued' | 'running' | 'success' | 'failed' | 'cancelled' | string;
  detail?: string;
};

export type UpdateQueueRequest = {
  candidates?: Array<{
    appId: number;
    sourceId: number;
    packageId: string;
    installedVersion: string;
    targetVersion: string;
  }>;
  mirrorOverrides?: Array<{
    sourceId: number;
    downloadMirrorId: string;
    rawMirrorId: string;
  }>;
};

export type UpdateQueueResult = {
  status: 'running' | 'success' | 'partial' | 'failed' | 'cancelled' | 'no_updates' | 'already_running' | string;
  items?: UpdateQueueItem[];
  error?: string;
};

export type ChatParticipant = {
  actorType: 'USER' | 'CLIENT' | string;
  userId?: number;
  clientUserId?: string;
  displayName: string;
  avatarUrl?: string;
  isSelf?: boolean;
};

export type ChatConversation = {
  id: number;
  appId?: number;
  appName?: string;
  topic: string;
  origin: string;
  sourceId?: SourceID;
  sourceName?: string;
  participants: ChatParticipant[];
  peer?: ChatParticipant;
  lastMessageBody?: string;
  lastMessageSenderName?: string;
  lastMessageAt?: string;
  unreadCount?: number;
  createdAt: string;
  updatedAt: string;
};

export type ChatMessage = {
  id: number;
  conversationId: number;
  senderType: 'USER' | 'CLIENT' | string;
  senderUserId?: number;
  senderClientUserId?: string;
  senderName: string;
  senderAvatarUrl?: string;
  body: string;
  isMine: boolean;
  createdAt: string;
};

export type InstallHistoryEntry = {
  id: number;
  sourceId?: SourceID;
  sourceAppId?: number;
  sourceName?: string;
  packageId: string;
  appName: string;
  version?: string;
  result: 'SUCCESS' | 'FAILED';
  downloadUrl?: string;
  sha256?: string;
  error?: string;
  createdAt: string;
};

export type ThemeMode = 'system' | 'light' | 'dark';
export type ResolvedTheme = Exclude<ThemeMode, 'system'>;
export type SortMode = 'recent' | 'downloads' | 'downloads_day' | 'downloads_week' | 'downloads_month' | 'downloads_year' | 'name';
export type SourceAppFilter = 'all' | 'installable' | 'installed' | 'updates' | 'incomplete';
export type SourceHealth = 'syncing' | 'auth' | 'failed' | 'stale' | 'synced' | 'unsynced';
export type SourceHealthFilter = 'all' | Exclude<SourceHealth, 'syncing'>;
export type CollectionDraft = { name: string; slug: string; kind: string; appIds: number[] };
export type SourceActionKey = 'install' | 'reinstall' | 'update' | 'unavailable';
