import type { ReactNode } from 'react';
import { SelectableCard as XSelectableCard } from '@astryxdesign/core/SelectableCard';
import { HStack as XHStack, VStack as XVStack } from '@astryxdesign/core/Stack';
import { Text as XText } from '@astryxdesign/core/Text';

export function ArtifactModeOption({
  icon,
  title,
  hint,
  isSelected,
  onSelect,
}: {
  icon: ReactNode;
  title: string;
  hint: string;
  isSelected: boolean;
  onSelect: () => void;
}) {
  return (
    <XSelectableCard label={title} isSelected={isSelected} onChange={onSelect} padding={3} height="100%">
      <XHStack gap={2} align="start">
        {icon}
        <XVStack gap={1}>
          <XText type="body" weight="semibold" display="block" wordBreak="break-word">
            {title}
          </XText>
          <XText type="supporting" color="secondary" display="block" wordBreak="break-word">
            {hint}
          </XText>
        </XVStack>
      </XHStack>
    </XSelectableCard>
  );
}
