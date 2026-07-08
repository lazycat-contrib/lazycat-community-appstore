import { useState } from 'react';
import { importMigrationPackage, previewMigrationPackage } from './api';
import { defaultMigrationOptions } from './constants';
import type { ApiClient, MigrationImportMode, MigrationImportResult, MigrationModuleOptions, MigrationPreview } from './types';

export function useMigrationImport(api: ApiClient) {
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<MigrationPreview | null>(null);
  const [options, setOptions] = useState<MigrationModuleOptions>(defaultMigrationOptions);
  const [mode, setMode] = useState<MigrationImportMode>('merge');
  const [isPreviewing, setIsPreviewing] = useState(false);
  const [isImporting, setIsImporting] = useState(false);
  const [replaceDialogOpen, setReplaceDialogOpen] = useState(false);
  const [result, setResult] = useState<MigrationImportResult | null>(null);

  function changeFile(next: File | null) {
    setFile(next);
    setPreview(null);
    setResult(null);
  }

  function setOption(key: keyof MigrationModuleOptions, value: boolean) {
    setOptions((current) => {
      const next = { ...current, [key]: value };
      if (key === 'includeFiles' && value) next.includeApps = true;
      if (key === 'includeApps' && !value) next.includeFiles = false;
      return next;
    });
  }

  async function previewPackage() {
    if (!file) return;
    setIsPreviewing(true);
    try {
      const nextPreview = await previewMigrationPackage(api, file);
      setPreview(nextPreview);
      setOptions({
        includeSite: nextPreview.modules.includes('site'),
        includePeople: nextPreview.modules.includes('people'),
        includeApps: nextPreview.modules.includes('apps'),
        includeFiles: nextPreview.modules.includes('files'),
      });
    } finally {
      setIsPreviewing(false);
    }
  }

  async function applyImport(confirmReplace?: string) {
    if (!file || !preview) return null;
    setIsImporting(true);
    try {
      const nextResult = await importMigrationPackage(api, {
        file,
        options,
        mode,
        confirmReplace: mode === 'replace' ? confirmReplace : undefined,
      });
      setResult(nextResult);
      setReplaceDialogOpen(false);
      return nextResult;
    } finally {
      setIsImporting(false);
    }
  }

  return {
    file,
    preview,
    options,
    mode,
    result,
    isPreviewing,
    isImporting,
    replaceDialogOpen,
    setMode,
    setOption,
    setReplaceDialogOpen,
    changeFile,
    previewPackage,
    applyImport,
  };
}
