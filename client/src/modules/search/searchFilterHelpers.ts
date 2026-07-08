import type { EnumItem, PowerSearchFilter } from '@astryxdesign/core/PowerSearch';

function normalize(value: string) {
  return value.trim().toLowerCase();
}

export function uniqueEnumOptions(values: string[]): EnumItem[] {
  const seen = new Set<string>();
  const items: EnumItem[] = [];
  values.forEach((value) => {
    const label = value.trim();
    const key = normalize(label);
    if (!label || seen.has(key)) return;
    seen.add(key);
    items.push({ value: key, label });
  });
  return items;
}

export function enumOptionsFromEntries(entries: Array<{ value: string; label: string }>): EnumItem[] {
  const seen = new Set<string>();
  const items: EnumItem[] = [];
  entries.forEach((entry) => {
    const value = entry.value.trim();
    const key = normalize(value);
    if (!value || seen.has(key)) return;
    seen.add(key);
    items.push({ value, label: entry.label });
  });
  return items;
}

export function matchesStringFilter(text: string, filter: PowerSearchFilter) {
  if (filter.value.type !== 'string') return true;
  const target = normalize(filter.value.value);
  if (!target) return true;
  const source = text.toLowerCase();
  switch (filter.operator) {
    case 'not_contains':
      return !source.includes(target);
    case 'starts_with':
      return source.startsWith(target);
    case 'not_starts_with':
      return !source.startsWith(target);
    case 'ends_with':
      return source.endsWith(target);
    case 'not_ends_with':
      return !source.endsWith(target);
    case 'is':
      return source === target;
    case 'is_not':
      return source !== target;
    case 'contains':
    default:
      return source.includes(target);
  }
}

export function matchesChoiceFilter(value: string | string[], filter: PowerSearchFilter) {
  const values = (Array.isArray(value) ? value : [value]).map(normalize).filter(Boolean);
  if (filter.value.type === 'enum') {
    const target = normalize(filter.value.value);
    if (!target) return true;
    const hasValue = values.includes(target);
    return filter.operator === 'is_not' ? !hasValue : hasValue;
  }
  if (filter.value.type === 'enum_list') {
    const targets = filter.value.value.map(normalize).filter(Boolean);
    if (targets.length === 0) return true;
    const hasAny = values.some((item) => targets.includes(item));
    return filter.operator === 'is_none_of' ? !hasAny : hasAny;
  }
  return true;
}

export function filterSignature(filters: ReadonlyArray<PowerSearchFilter>) {
  return JSON.stringify(filters);
}
