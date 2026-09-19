import { useState } from 'react';
import { CheckboxInput as XCheckboxInput } from '@astryxdesign/core/CheckboxInput';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { localizedText } from '../../shared/utils';
import { filterCollectionApps } from './collectionAppPickerState';

type CollectionAppOption = {
  id: number;
  name: string;
  nameI18n?: Record<string, string>;
  slug: string;
  packageId?: string;
};

type CollectionAppPickerLabels = {
  title: string;
  selectedCount: string;
  empty: string;
  searchPlaceholder: string;
  noResults: string;
};

function toggleAppSelection(appIds: number[], appID: number, checked: boolean) {
  if (checked) return appIds.includes(appID) ? appIds : [...appIds, appID];
  return appIds.filter((id) => id !== appID);
}

export function CollectionAppPicker({
  apps,
  appIds,
  labels,
  onChange,
}: {
  apps: CollectionAppOption[];
  appIds: number[];
  labels: CollectionAppPickerLabels;
  onChange: (appIds: number[]) => void;
}) {
  const [query, setQuery] = useState('');
  const filteredApps = filterCollectionApps(apps, query);

  return (
    <div className="collection-picker" role="group" aria-label={labels.title}>
      <div className="collection-picker-head">
        <strong>{labels.title}</strong>
        <span>{labels.selectedCount}</span>
      </div>
      {apps.length === 0 ? (
        <p className="field-help">{labels.empty}</p>
      ) : (
        <>
          <div className="collection-picker-search">
            <XTextInput
              label={labels.searchPlaceholder}
              isLabelHidden
              placeholder={labels.searchPlaceholder}
              value={query}
              onChange={setQuery}
            />
          </div>
          {filteredApps.length === 0 ? (
            <p className="field-help collection-picker-empty">{labels.noResults}</p>
          ) : (
            <div className="collection-app-options">
              {filteredApps.map((app) => (
                <XCheckboxInput
                  key={app.id}
                  label={localizedText(app.nameI18n, app.name)}
                  description={app.packageId || app.slug}
                  value={appIds.includes(app.id)}
                  onChange={(checked) => onChange(toggleAppSelection(appIds, app.id, checked))}
                />
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}
