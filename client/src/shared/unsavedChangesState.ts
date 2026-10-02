export type UnsavedChange = {
  isDirty: boolean;
  save?: () => Promise<boolean>;
  discard: () => void;
};

export async function settleUnsavedChanges(changes: UnsavedChange[], action: 'save' | 'discard') {
  for (const change of changes) {
    if (!change.isDirty) continue;
    if (action === 'discard') change.discard();
    else if (!change.save || !await change.save()) return false;
  }
  return true;
}
