import { CheckboxInput as XCheckboxInput } from '@astryxdesign/core/CheckboxInput';

type CollectionAppOption = {
  id: number;
  name: string;
  slug: string;
  packageId?: string;
};

type CollectionAppPickerLabels = {
  title: string;
  selectedCount: string;
  empty: string;
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
  return (
    <div className="collection-picker" role="group" aria-label={labels.title}>
      <div className="collection-picker-head">
        <strong>{labels.title}</strong>
        <span>{labels.selectedCount}</span>
      </div>
      {apps.length === 0 ? (
        <p className="field-help">{labels.empty}</p>
      ) : (
        <div className="collection-app-options">
          {apps.map((app) => (
            <XCheckboxInput
              key={app.id}
              label={app.name}
              description={app.packageId || app.slug}
              value={appIds.includes(app.id)}
              onChange={(checked) => onChange(toggleAppSelection(appIds, app.id, checked))}
            />
          ))}
        </div>
      )}
    </div>
  );
}
