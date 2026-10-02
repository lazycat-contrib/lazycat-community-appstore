import assert from 'node:assert/strict';
import test from 'node:test';
import { fileMatchesAccept } from './filePickerState.ts';

test('LPK extension filters support files without a MIME type', () => {
  assert.equal(fileMatchesAccept({name:'app.LPK',type:''},'.lpk'),true);
  assert.equal(fileMatchesAccept({name:'app.txt',type:''},'.lpk'),false);
});
test('image filters allow MIME families and comma-separated exact types', () => {
  assert.equal(fileMatchesAccept({name:'photo.png',type:'image/png'},'image/*'),true);
  assert.equal(fileMatchesAccept({name:'photo.jpg',type:'image/jpeg'},'image/png, image/jpeg'),true);
  assert.equal(fileMatchesAccept({name:'script.js',type:'text/javascript'},'image/*'),false);
});
test('omitted or blank filters accept any file', () => {
  assert.equal(fileMatchesAccept({name:'file.bin',type:''}),true);
  assert.equal(fileMatchesAccept({name:'file.bin',type:''},' '),true);
});
