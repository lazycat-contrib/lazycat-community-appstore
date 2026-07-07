import { Avatar as HumationAvatar } from '@humation/react';
import { humation1 } from '@humation/assets-humation-1';
import { useEffect, useState } from 'react';
import type { User } from '../shared/types';

function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(' ');
}

export function AvatarIcon({ seed, title, size = 46, className }: { seed: string; title?: string; size?: number; className?: string }) {
  return (
    <HumationAvatar
      assets={humation1}
      seed={seed || title || 'lazycat-app'}
      size={size}
      className={cx('humation-avatar', className)}
      title={title}
      aria-hidden={title ? undefined : true}
    />
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

export function UserAvatar({ user, size = 40, className, decorative = true }: { user: User; size?: number; className?: string; decorative?: boolean }) {
  const displayName = user.nickname || user.username;
  return <AppIcon src={user.avatarUrl} seed={user.email || user.username} title={decorative ? undefined : displayName} size={size} className={className} />;
}
