import type { StorageOption, StoreApp, User } from './types';

export function canUserManageApp(user: User | null | undefined, app: StoreApp) {
  if (!user) return false;
  return app.canManageApp ?? (user.role === 'SITE_ADMIN' || user.role === 'SOFTWARE_ADMIN' || user.id === app.ownerId);
}

export function canUserUploadVersion(user: User | null | undefined, app: StoreApp) {
  return !!user && (app.canUploadVersion || canUserManageApp(user, app));
}

export function screenshotFileKey(file: File) {
  return `${file.name}:${file.size}:${file.lastModified}`;
}

export function reconcileScreenshotCaptions(files: File[], current: Record<string, string>) {
  return Object.fromEntries(files.map((file) => {
    const key = screenshotFileKey(file);
    return [key, current[key] || ''];
  }));
}

export function displayUserName(user: User | null | undefined) {
  return user?.nickname?.trim() || user?.username || '';
}

export function storageSelectOptions(storages: StorageOption[]) {
  return storages.map((storage) => ({
    value: storage.key,
    label: storage.name || storage.key,
  }));
}

export function defaultUploadStorageKey(storages: StorageOption[]) {
  return storages.find((storage) => storage.isDefault)?.key || storages[0]?.key || 'primary';
}
