import assert from 'node:assert/strict';
import test from 'node:test';
import { addGroupCodeBatch } from './sourceConfig.ts';

test('group code batch input accepts common separators and normalizes case', () => {
  assert.deepEqual(addGroupCodeBatch('abc123, DEF456\nGHI789；JKL012', []), {
    codes: ['ABC123', 'DEF456', 'GHI789', 'JKL012'],
    pending: [],
    addedCount: 4,
    duplicateCount: 0,
    invalidCount: 0,
    overflowCount: 0,
  });
});

test('group code batch input keeps invalid values editable and skips duplicates', () => {
  assert.deepEqual(addGroupCodeBatch('abc123 invalid DEF456', ['ABC123']), {
    codes: ['ABC123', 'DEF456'],
    pending: ['invalid'],
    addedCount: 1,
    duplicateCount: 1,
    invalidCount: 1,
    overflowCount: 0,
  });
});

test('group code batch input enforces the server limit without losing overflow values', () => {
  assert.deepEqual(addGroupCodeBatch('DEF456 GHI789', ['ABC123'], 2), {
    codes: ['ABC123', 'DEF456'],
    pending: ['GHI789'],
    addedCount: 1,
    duplicateCount: 0,
    invalidCount: 0,
    overflowCount: 1,
  });
});

test('group code batch input never truncates pre-existing values above the current limit', () => {
  assert.deepEqual(addGroupCodeBatch('ZZZ999', ['ABC123', 'DEF456', 'GHI789'], 2), {
    codes: ['ABC123', 'DEF456', 'GHI789'],
    pending: ['ZZZ999'],
    addedCount: 0,
    duplicateCount: 0,
    invalidCount: 0,
    overflowCount: 1,
  });
});
