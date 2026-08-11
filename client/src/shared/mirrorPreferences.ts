import type { GitHubMirrorOption } from './types';

export const PREFERRED_MIRROR_SELECTION = '__preferred__';

export function orderGitHubMirrors<T extends GitHubMirrorOption>(mirrors: T[], preferredID = ''): T[] {
  const pinnedID = preferredID.trim();
  return mirrors
    .map((mirror, index) => ({ mirror, index }))
    .sort((left, right) => {
      const leftMeasured = Boolean(left.mirror.benchmarkStatus || left.mirror.speedBytesPerSecond);
      const rightMeasured = Boolean(right.mirror.benchmarkStatus || right.mirror.speedBytesPerSecond);
      const leftRank = left.mirror.benchmarkStatus === 'unavailable' ? 2 : leftMeasured ? 0 : 1;
      const rightRank = right.mirror.benchmarkStatus === 'unavailable' ? 2 : rightMeasured ? 0 : 1;
      if (leftRank !== rightRank) return leftRank - rightRank;
      const leftSpeed = Number(left.mirror.speedBytesPerSecond) || 0;
      const rightSpeed = Number(right.mirror.speedBytesPerSecond) || 0;
      if (leftSpeed !== rightSpeed) return rightSpeed - leftSpeed;
      const leftStability = Number(left.mirror.stabilityPercent) || 0;
      const rightStability = Number(right.mirror.stabilityPercent) || 0;
      if (leftStability !== rightStability) return rightStability - leftStability;
      const leftScore = Number(left.mirror.benchmarkScore) || 0;
      const rightScore = Number(right.mirror.benchmarkScore) || 0;
      if (leftScore !== rightScore) return rightScore - leftScore;
      const leftPinned = left.mirror.id === pinnedID ? 0 : 1;
      const rightPinned = right.mirror.id === pinnedID ? 0 : 1;
      return leftPinned - rightPinned
        || left.index - right.index
        || left.mirror.id.localeCompare(right.mirror.id);
    })
    .map(({ mirror }) => mirror);
}

export function resolveMirrorSelection<T extends GitHubMirrorOption>(selectionID: string, orderedMirrors: T[]): string {
  if (selectionID !== PREFERRED_MIRROR_SELECTION) return selectionID;
  return orderedMirrors[0]?.id || '';
}
