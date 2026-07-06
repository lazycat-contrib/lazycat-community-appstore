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
            <label className="toggle-line" key={app.id}>
              <input
                type="checkbox"
                checked={appIds.includes(app.id)}
                onChange={(event) => onChange(toggleAppSelection(appIds, app.id, event.target.checked))}
              />
              <span>{app.name}</span>
              <small>{app.packageId || app.slug}</small>
            </label>
          ))}
        </div>
      )}
    </div>
  );
}
