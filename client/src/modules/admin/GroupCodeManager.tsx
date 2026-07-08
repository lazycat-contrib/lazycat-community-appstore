import { Copy, RefreshCw } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { useTranslation } from 'react-i18next';
import type { Group, Toast } from '../../shared/types';
import { errorMessage } from '../../shared/utils';

export function GroupCodeManager({
  group,
  onRotate,
  setToast,
}: {
  group: Group;
  onRotate: (group: Group) => Promise<void>;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();

  async function copyCode() {
    try {
      if (!group.code || !navigator.clipboard?.writeText) throw new Error(t('home.copySourceUnsupported'));
      await navigator.clipboard.writeText(group.code);
      setToast({ tone: 'success', message: t('admin.groups.codeCopied') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.groups.codeCopyFailed')) });
    }
  }

  return (
    <div className="group-code-manager">
      <code>{group.code || '------'}</code>
      <XButton type="button" size="sm" variant="secondary" label={t('admin.groups.copyCode')} icon={<Copy size={16} />} onClick={() => void copyCode()} />
      <XButton type="button" size="sm" variant="secondary" label={t('admin.groups.rotateCode')} icon={<RefreshCw size={16} />} onClick={() => void onRotate(group)} />
    </div>
  );
}
