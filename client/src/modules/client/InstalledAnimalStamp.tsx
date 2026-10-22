import {
  ArrowUp, Bird, Bug, Cat, Check, createLucideIcon, Dog, Fish, Mouse,
  Panda, Rabbit, Rat, Shrimp, Snail, Squirrel, Turtle, Worm,
} from 'lucide-react';

const Cow = createLucideIcon('Cow', [
  ['path', { d: 'M5 8 3 5l3-1 2 2a8 8 0 0 1 8 0l2-2 3 1-2 3', key: 'horns' }],
  ['path', { d: 'M5 8v6a7 7 0 0 0 14 0V8', key: 'head' }],
  ['path', { d: 'M8 15c0-2 2-3 4-3s4 1 4 3v2c0 2-2 3-4 3s-4-1-4-3z', key: 'muzzle' }],
  ['path', { d: 'M9 9h.01M15 9h.01M10 16h.01M14 16h.01', key: 'face' }],
]);
const Fox = createLucideIcon('Fox', [
  ['path', { d: 'm4 3 5 3a8 8 0 0 1 6 0l5-3-1 9c0 5-3 9-7 9s-7-4-7-9z', key: 'head' }],
  ['path', { d: 'm5 12 5 4h4l5-4M9 11h.01M15 11h.01m-3 5 0 2', key: 'face' }],
]);
const Owl = createLucideIcon('Owl', [
  ['path', { d: 'M4 4 8 6a8 8 0 0 1 8 0l4-2v10a8 8 0 0 1-16 0z', key: 'head' }],
  ['circle', { cx: '9', cy: '12', r: '2', key: 'left-eye' }],
  ['circle', { cx: '15', cy: '12', r: '2', key: 'right-eye' }],
  ['path', { d: 'm11 16 1 2 1-2', key: 'beak' }],
]);
const Pig = createLucideIcon('Pig', [
  ['path', { d: 'M5 8 3 5l4-1 2 2a8 8 0 0 1 6 0l2-2 4 1-2 3a8 8 0 1 1-14 0z', key: 'head' }],
  ['path', { d: 'M9 15a3 2 0 0 1 6 0 3 2 0 0 1-6 0zM9 10h.01M15 10h.01M11 15h.01M13 15h.01', key: 'face' }],
]);

const animals = [
  Bird, Bug, Cat, Cow, Dog, Fish, Fox, Mouse, Owl,
  Panda, Pig, Rabbit, Rat, Shrimp, Snail, Squirrel, Turtle, Worm,
] as const;

const animalHeads = [Cat, Cow, Dog, Fox, Mouse, Owl, Panda, Pig, Rabbit] as const;

export function installedAnimalIndex(identity: string) {
  let hash = 2166136261;
  for (const char of identity) hash = Math.imul(hash ^ char.codePointAt(0)!, 16777619);
  return (hash >>> 0) % animals.length;
}

export function AnimalGlyph({ identity, size = 18, className, headOnly = false }: { identity: string; size?: number; className?: string; headOnly?: boolean }) {
  const index = installedAnimalIndex(identity);
  const Animal = headOnly ? animalHeads[index % animalHeads.length] : animals[index];
  return <Animal className={className} size={size} strokeWidth={2} />;
}

export function InstalledAnimalStamp({ identity, updateAvailable, label, celebrate }: {
  identity: string;
  updateAvailable: boolean;
  label: string;
  celebrate: boolean;
}) {
  return (
    <span
      className={`installed-animal-stamp${updateAvailable ? ' has-update' : ''}${celebrate ? ' is-celebrating' : ''}`}
      title={label}
      aria-hidden="true"
    >
      <AnimalGlyph className="installed-animal" identity={identity} />
      <span className="installed-animal-state">
        {updateAvailable ? <ArrowUp size={10} strokeWidth={3} /> : <Check size={10} strokeWidth={3} />}
      </span>
    </span>
  );
}
