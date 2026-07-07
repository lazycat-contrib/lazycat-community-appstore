import { Tokenizer as XTokenizer } from '@astryxdesign/core/Tokenizer';
import { createStaticSource, type SearchableItem } from '@astryxdesign/core/Typeahead';
import { useMemo } from 'react';

type TagItem = SearchableItem;

function splitTags(value: string) {
  return value
    .split(',')
    .map((tag) => tag.trim())
    .filter(Boolean);
}

function tagItem(label: string): TagItem {
  return { id: label.toLowerCase(), label };
}

function uniqueTags(tags: string[]) {
  const seen = new Set<string>();
  const out: string[] = [];
  tags.forEach((tag) => {
    const normalized = tag.trim();
    const key = normalized.toLowerCase();
    if (!normalized || seen.has(key)) return;
    seen.add(key);
    out.push(normalized);
  });
  return out;
}

export function TagTokenizer({
  label,
  value,
  knownTags,
  onChange,
}: {
  label: string;
  value: string;
  knownTags: string[];
  onChange: (value: string) => void;
}) {
  const selectedTags = useMemo(() => uniqueTags(splitTags(value)), [value]);
  const sourceItems = useMemo(() => uniqueTags([...knownTags, ...selectedTags]).map(tagItem), [knownTags, selectedTags]);
  const selectedItems = useMemo(() => selectedTags.map(tagItem), [selectedTags]);
  const searchSource = useMemo(() => createStaticSource(sourceItems), [sourceItems]);

  return (
    <XTokenizer
      label={label}
      searchSource={searchSource}
      value={selectedItems}
      hasCreate
      hasClear
      hasEntriesOnFocus
      debounceMs={0}
      width="100%"
      onChange={(items) => onChange(uniqueTags(items.map((item) => item.label)).join(', '))}
    />
  );
}
