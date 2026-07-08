import { Archive, DatabaseBackup, Files, PackageCheck, Users } from 'lucide-react';
import type { MigrationModuleKey, MigrationModuleOptions } from './types';

export const defaultMigrationOptions: MigrationModuleOptions = {
  includeSite: true,
  includePeople: true,
  includeApps: true,
  includeFiles: true,
};

export const migrationModules: Array<{
  key: MigrationModuleKey;
  optionKey: keyof MigrationModuleOptions;
  icon: typeof Archive;
  labelKey: string;
  descriptionKey: string;
}> = [
  { key: 'site', optionKey: 'includeSite', icon: DatabaseBackup, labelKey: 'admin.migration.modules.site', descriptionKey: 'admin.migration.moduleDescriptions.site' },
  { key: 'people', optionKey: 'includePeople', icon: Users, labelKey: 'admin.migration.modules.people', descriptionKey: 'admin.migration.moduleDescriptions.people' },
  { key: 'apps', optionKey: 'includeApps', icon: PackageCheck, labelKey: 'admin.migration.modules.apps', descriptionKey: 'admin.migration.moduleDescriptions.apps' },
  { key: 'files', optionKey: 'includeFiles', icon: Files, labelKey: 'admin.migration.modules.files', descriptionKey: 'admin.migration.moduleDescriptions.files' },
];

export const importModeOptions = [
  { value: 'merge', labelKey: 'admin.migration.mergeMode' },
  { value: 'replace', labelKey: 'admin.migration.replaceMode' },
] as const;

export const replaceConfirmationText = 'OVERWRITE';
