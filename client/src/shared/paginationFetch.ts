type PaginationMeta = {
  totalItems: number;
  totalPages: number;
};

type PaginatedPage<TItem, TKey extends string> = Record<TKey, TItem[]> & {
  pagination?: PaginationMeta;
};

export async function fetchAllPaginated<TItem, TKey extends string>(
  request: <T>(path: string) => Promise<T>,
  path: string,
  key: TKey,
  pageSize = 100,
  identity?: (item: TItem) => string | number,
): Promise<TItem[]> {
  const maxAttempts = identity ? 2 : 1;
  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    const items: TItem[] = [];
    let expectedTotal: number | undefined;
    let page = 1;
    for (;;) {
      const separator = path.includes('?') ? '&' : '?';
      const data = await request<PaginatedPage<TItem, TKey>>(`${path}${separator}page=${page}&pageSize=${pageSize}`);
      const pageItems = data[key] || [];
      items.push(...pageItems);
      expectedTotal = data.pagination?.totalItems ?? expectedTotal;
      if (!data.pagination || page >= data.pagination.totalPages || pageItems.length === 0) break;
      page += 1;
    }

    if (!identity) return items;
    const uniqueItems = Array.from(new Map(items.map((item) => [identity(item), item])).values());
    if (expectedTotal === undefined || uniqueItems.length === expectedTotal) return uniqueItems;
  }
  throw new Error('Paginated result changed while loading');
}
