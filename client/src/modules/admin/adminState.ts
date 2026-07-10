export type AdminSaveStatus = 'idle' | 'dirty' | 'saving' | 'saved' | 'error';

export type AdminStorageAction = 'create' | 'save' | 'test' | 'default' | 'delete' | null;

export type AdminOperationResult = {
  variant: 'neutral' | 'success' | 'warning' | 'error' | 'info';
  title: string;
  message: string;
  occurredAt: string;
  target?: string;
};

type AdminDraftPrimitive = boolean | null | number | string;
type AdminJsonValue =
  | AdminDraftPrimitive
  | readonly AdminJsonValue[]
  | { readonly [key: string]: AdminJsonValue | undefined };

/**
 * A normalized admin form draft: JSON-compatible data or `undefined` for an
 * absent whole draft. Optional object-property `undefined` values are treated
 * as omitted. Arrays must be dense and may contain only JSON values; numbers
 * must be finite; objects must be plain and acyclic.
 */
export type AdminDraftValue = AdminJsonValue | undefined;

function invalidAdminDraft(path: string): never {
  throw new TypeError(`areAdminDraftsEqual only accepts JSON-compatible admin drafts; invalid value at ${path}`);
}

function compareCodeUnits(left: string, right: string): number {
  if (left < right) return -1;
  if (left > right) return 1;
  return 0;
}

function canonicalize(value: unknown, ancestors = new Set<object>(), path = '$'): unknown {
  if (value === undefined || value === null || typeof value === 'string' || typeof value === 'boolean') {
    return value;
  }
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : invalidAdminDraft(path);
  }
  if (Array.isArray(value)) {
    if (ancestors.has(value)) return invalidAdminDraft(path);
    ancestors.add(value);
    try {
      return Array.from({ length: value.length }, (_, index) => {
        if (!Object.hasOwn(value, index) || value[index] === undefined) {
          return invalidAdminDraft(`${path}[${index}]`);
        }
        return canonicalize(value[index], ancestors, `${path}[${index}]`);
      });
    } finally {
      ancestors.delete(value);
    }
  }
  if (typeof value === 'object') {
    const prototype = Object.getPrototypeOf(value);
    if ((prototype !== Object.prototype && prototype !== null) || Object.getOwnPropertySymbols(value).length > 0) {
      return invalidAdminDraft(path);
    }
    if (ancestors.has(value)) return invalidAdminDraft(path);
    ancestors.add(value);
    try {
      return Object.fromEntries(
        Object.entries(value)
          .filter(([, item]) => item !== undefined)
          .sort(([left], [right]) => compareCodeUnits(left, right))
          .map(([key, item]) => [key, canonicalize(item, ancestors, `${path}.${key}`)]),
      );
    } finally {
      ancestors.delete(value);
    }
  }
  return invalidAdminDraft(path);
}

export function areAdminDraftsEqual(left: unknown, right: unknown): boolean {
  return JSON.stringify(canonicalize(left)) === JSON.stringify(canonicalize(right));
}
