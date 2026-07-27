import assert from 'node:assert/strict';
import test from 'node:test';

test('full pagination retries when a changing sort produces duplicate and missing apps', async () => {
  const { fetchAllPaginated } = await import('./paginationFetch.ts');
  const responses = [
    { apps: [{ id: 1 }, { id: 2 }], pagination: { page: 1, pageSize: 2, totalItems: 3, totalPages: 2 } },
    { apps: [{ id: 2 }], pagination: { page: 2, pageSize: 2, totalItems: 3, totalPages: 2 } },
    { apps: [{ id: 1 }, { id: 2 }], pagination: { page: 1, pageSize: 2, totalItems: 3, totalPages: 2 } },
    { apps: [{ id: 3 }], pagination: { page: 2, pageSize: 2, totalItems: 3, totalPages: 2 } },
  ];
  const requestedPaths = [];
  const request = async (path) => {
    requestedPaths.push(path);
    return responses.shift();
  };

  const apps = await fetchAllPaginated(request, '/api/v1/apps', 'apps', 2, (app) => app.id);

  assert.deepEqual(apps.map((app) => app.id), [1, 2, 3]);
  assert.deepEqual(requestedPaths, [
    '/api/v1/apps?page=1&pageSize=2',
    '/api/v1/apps?page=2&pageSize=2',
    '/api/v1/apps?page=1&pageSize=2',
    '/api/v1/apps?page=2&pageSize=2',
  ]);
});
