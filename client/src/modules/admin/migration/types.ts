export type MigrationModuleKey = 'site' | 'people' | 'apps' | 'files';

export type MigrationModuleOptions = {
  includeSite: boolean;
  includePeople: boolean;
  includeApps: boolean;
  includeFiles: boolean;
};

export type MigrationImportMode = 'merge' | 'replace';

export type MigrationPreview = {
  formatVersion: number;
  serverVersion: string;
  createdAt: string;
  modules: MigrationModuleKey[];
  counts: Record<string, number>;
  totalFileBytes: number;
  warnings?: string[];
};

export type MigrationImportResult = {
  mode: MigrationImportMode;
  created: number;
  updated: number;
  skipped: number;
  warnings?: string[];
};

export type MigrationImportInput = {
  file: File;
  options: MigrationModuleOptions;
  mode: MigrationImportMode;
  confirmReplace?: string;
};

export type ApiClient = <T>(path: string, options?: RequestInit) => Promise<T>;
