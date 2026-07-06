import { type CSSProperties, useEffect, useState } from 'react';

function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(' ');
}

function hashSeed(seed: string) {
  let hash = 0;
  for (let index = 0; index < seed.length; index += 1) {
    hash = (hash * 31 + seed.charCodeAt(index)) >>> 0;
  }
  return hash;
}

function initialsFrom(seed: string, title?: string) {
  const value = (title || seed || 'App').trim();
  const parts = value.split(/[\s._-]+/).filter(Boolean);
  if (parts.length >= 2) return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
  return value.slice(0, 2).toUpperCase();
}

type AvatarStyle = CSSProperties & { '--avatar-hue': number };

export function AvatarIcon({ seed, title, size = 46, className }: { seed: string; title?: string; size?: number; className?: string }) {
  const hue = hashSeed(seed || title || 'lazycat-app') % 360;
  const style: AvatarStyle = { width: size, height: size, '--avatar-hue': hue };
  return (
    <span
      className={cx('humation-avatar', 'avatar-fallback', className)}
      title={title}
      style={style}
      aria-hidden="true"
    >
      {initialsFrom(seed, title)}
    </span>
  );
}

export function AppIcon({ src, seed, title, size = 46, className }: { src?: string; seed: string; title?: string; size?: number; className?: string }) {
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setFailed(false);
  }, [src]);

  if (src && !failed) {
    return (
      <img
        className={cx('app-artwork', className)}
        src={src}
        alt=""
        title={title}
        loading="lazy"
        style={{ width: size, height: size }}
        onError={() => setFailed(true)}
      />
    );
  }

  return <AvatarIcon seed={seed} title={title} size={size} className={className} />;
}
