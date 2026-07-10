import type { Version } from '../../shared/types';

export function retentionPruneCount(
  versions: Version[],
  effectiveMaxVersions: number,
): number {
  if (effectiveMaxVersions === 0) return 0;
  const approved = versions.filter((version) => version.status === 'APPROVED');
  return Math.max(0, approved.length - effectiveMaxVersions);
}

export function nextLatestVersion(
  versions: Version[],
  deletingID: number,
): Version | null {
  const approved = versions
    .filter(
      (version) =>
        version.status === 'APPROVED' && version.id !== deletingID,
    )
    .sort((left, right) => {
      const leftTime = Date.parse(left.publishedAt || left.createdAt);
      const rightTime = Date.parse(right.publishedAt || right.createdAt);
      return rightTime - leftTime || right.id - left.id;
    });
  return approved[0] || null;
}
