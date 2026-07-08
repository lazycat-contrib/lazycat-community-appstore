import { useMemo, useState } from 'react';
import { exportMigrationPackage } from './api';
import { defaultMigrationOptions } from './constants';
import type { ApiClient, MigrationModuleOptions } from './types';

export function useMigrationExport(api: ApiClient) {
  const [options, setOptions] = useState<MigrationModuleOptions>(defaultMigrationOptions);
  const [isExporting, setIsExporting] = useState(false);
  const hasSelection = useMemo(() => Object.values(options).some(Boolean), [options]);

  function setOption(key: keyof MigrationModuleOptions, value: boolean) {
    setOptions((current) => {
      const next = { ...current, [key]: value };
      if (key === 'includeFiles' && value) next.includeApps = true;
      if (key === 'includeApps' && !value) next.includeFiles = false;
      return next;
    });
  }

  async function exportPackage() {
    if (!hasSelection) return;
    setIsExporting(true);
    try {
      await exportMigrationPackage(api, options);
    } finally {
      setIsExporting(false);
    }
  }

  return { options, setOption, hasSelection, isExporting, exportPackage };
}
