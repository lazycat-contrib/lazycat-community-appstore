import assert from 'node:assert/strict';
import test from 'node:test';
import { settleUnsavedChanges } from './unsavedChangesState.ts';

test('a rejected save keeps navigation blocked and retains the draft', async () => {
  let discarded=false;
  assert.equal(await settleUnsavedChanges([{isDirty:true,save:async()=>false,discard:()=>{discarded=true;}}],'save'),false);
  assert.equal(discarded,false);
});

test('multiple settings saves serialize and stop at the first failure', async () => {
  const saved=[];
  const changes=[1,2,3].map(id=>({isDirty:true,save:async()=>{saved.push(id);return id!==2;},discard:()=>{}}));
  assert.equal(await settleUnsavedChanges(changes,'save'),false);
  assert.deepEqual(saved,[1,2]);
});

test('explicit discard resets each dirty section and allows navigation', async () => {
  const discarded=[];
  const changes=[1,2].map(id=>({isDirty:true,discard:()=>discarded.push(id)}));
  assert.equal(await settleUnsavedChanges(changes,'discard'),true);
  assert.deepEqual(discarded,[1,2]);
});

test('navigation can only proceed after every save succeeds', async () => {
  assert.equal(await settleUnsavedChanges([{isDirty:true,save:async()=>true,discard:()=>{}}],'save'),true);
  assert.equal(await settleUnsavedChanges([{isDirty:true,discard:()=>{}}],'save'),false);
});
