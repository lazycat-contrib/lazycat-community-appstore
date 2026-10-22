import { useState } from 'react';
import { AnimalGlyph } from './InstalledAnimalStamp';

export function ClientNavAnimal() {
  const [identity] = useState(() => Math.random().toString(36).slice(2));
  return (
    <span className="client-nav-pet" aria-hidden="true">
      <span className="client-nav-pet-reveal">
        <AnimalGlyph identity={identity} size={28} className="client-nav-pet-glyph" headOnly />
      </span>
    </span>
  );
}
