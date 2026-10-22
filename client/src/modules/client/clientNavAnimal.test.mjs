import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const source = (path) => readFile(new URL(path, import.meta.url), 'utf8');

test('client navigation reuses installation animals without changing identity on rerender', async () => {
  const [app, pet, stamp] = await Promise.all([
    source('../../App.tsx'), source('./ClientNavAnimal.tsx'), source('./InstalledAnimalStamp.tsx'),
  ]);
  assert.match(app, /endContent=\{!HAS_API \? <ClientNavAnimal \/> : undefined\}/);
  assert.match(pet, /useState\(\(\) => Math\.random\(\)/);
  assert.match(pet, /aria-hidden="true"/);
  assert.match(pet, /<AnimalGlyph identity=\{identity\}/);
  assert.match(pet, /className="client-nav-pet-glyph" headOnly/);
  assert.match(stamp, /<AnimalGlyph className="installed-animal" identity=\{identity\}/);
});

test('selected navigation pets use interruptible transitions and gated hover motion', async () => {
  const styles = await source('../../styles/client.css');
  assert.match(styles, /\.client-nav-pet-reveal \{[^}]*transition: transform 160ms var\(--ease-out\), opacity 160ms var\(--ease-out\)/);
  assert.match(styles, /@media \(hover: hover\) and \(pointer: fine\) \{[\s\S]*?\[aria-current='page'\]:hover \.client-nav-pet-glyph/);
  assert.match(styles, /html\[data-input-method='keyboard'\] :is\(\.client-nav-pet-reveal, \.client-nav-pet-glyph\) \{\s*transition: none/);
  assert.match(styles, /@media \(prefers-reduced-motion: reduce\) \{\s*\.client-nav-pet-reveal,[\s\S]*?transition: opacity 120ms ease/);
  assert.doesNotMatch(styles, /animation:[^;]*client-nav/);
});
