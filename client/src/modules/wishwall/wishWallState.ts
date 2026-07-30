export function mergeWishPage<T extends { id: number }>(current: T[], incoming: T[], append: boolean): T[] {
  if (!append) return incoming;
  return Array.from(new Map([...current, ...incoming].map((item) => [item.id, item])).values());
}

export function hasNextWishPage(page: number, totalPages: number): boolean {
  return page < totalPages;
}
