import type { LucideIcon } from 'lucide-react';
import { X } from 'lucide-react';
import { CodeBlock as XCodeBlock } from '@astryxdesign/core/CodeBlock';
import { Heading as XHeading } from '@astryxdesign/core/Heading';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Text as XText } from '@astryxdesign/core/Text';
import { useTranslation } from 'react-i18next';
import { ModalLayer } from './ModalLayer';

export type TokenHelpExample = {
  title: string;
  body: string;
  code: string;
  language: string;
};

export function TokenHelpDialog({
  icon: Icon,
  title,
  body,
  titleId,
  examples,
  onClose,
}: {
  icon: LucideIcon;
  title: string;
  body: string;
  titleId: string;
  examples: TokenHelpExample[];
  onClose: () => void;
}) {
  const { t } = useTranslation();

  return (
    <ModalLayer onClose={onClose} width="min(760px, calc(100vw - 36px))" maxHeight="min(86vh, 780px)">
      <section className="modal-panel token-help-panel" aria-labelledby={titleId}>
        <XIconButton type="button" className="close" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={onClose} />
        <div className="token-help-head">
          <span className="install-password-icon">
            <Icon size={21} />
          </span>
          <div>
            <XHeading id={titleId} level={2}>
              {title}
            </XHeading>
            <XText type="supporting" as="p" display="block" wordBreak="break-word">
              {body}
            </XText>
          </div>
        </div>
        <div className="token-help-content">
          {examples.map((example) => (
            <TokenHelpExampleSection key={example.title} {...example} />
          ))}
        </div>
      </section>
    </ModalLayer>
  );
}

function TokenHelpExampleSection({ title, body, code, language }: TokenHelpExample) {
  return (
    <section className="token-help-section">
      <div>
        <XText type="label" display="block" weight="semibold" wordBreak="break-word">
          {title}
        </XText>
        <XText type="supporting" as="p" display="block" wordBreak="break-word">
          {body}
        </XText>
      </div>
      <XCodeBlock code={code} language={language} width="100%" size="sm" />
    </section>
  );
}
