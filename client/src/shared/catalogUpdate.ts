export type CatalogUpdateRecord = {
  updatedAt?: string;
  latestVersion?: {
    publishedAt?: string;
    createdAt?: string;
  };
};

function validTimestamp(value?: string) {
  if (!value) return undefined;
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? timestamp : undefined;
}

export function softwareUpdatedAt(app: CatalogUpdateRecord) {
  const candidates = [app.latestVersion?.publishedAt, app.latestVersion?.createdAt, app.updatedAt];
  return candidates.find((value) => validTimestamp(value) !== undefined);
}

export function softwareUpdatedAtMillis(app: CatalogUpdateRecord) {
  return validTimestamp(softwareUpdatedAt(app)) ?? 0;
}
