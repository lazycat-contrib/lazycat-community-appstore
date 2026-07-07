import { useEffect, useState } from 'react';
import type { Screenshot } from './types';

export function usePreferredScreenshotDevice() {
  const readDevice = () => (window.matchMedia('(max-width: 720px)').matches ? 'MOBILE' : 'DESKTOP');
  const [device, setDevice] = useState<'DESKTOP' | 'MOBILE'>(readDevice);
  useEffect(() => {
    const query = window.matchMedia('(max-width: 720px)');
    const update = () => setDevice(query.matches ? 'MOBILE' : 'DESKTOP');
    update();
    query.addEventListener('change', update);
    return () => query.removeEventListener('change', update);
  }, []);
  return device;
}

export function orderedScreenshots(screenshots: Screenshot[] | undefined, preferredDevice: 'DESKTOP' | 'MOBILE') {
  return [...(screenshots || [])].sort((left, right) => {
    const leftPreferred = (left.deviceType || 'DESKTOP') === preferredDevice ? 0 : 1;
    const rightPreferred = (right.deviceType || 'DESKTOP') === preferredDevice ? 0 : 1;
    if (leftPreferred !== rightPreferred) return leftPreferred - rightPreferred;
    return (left.sortOrder || 0) - (right.sortOrder || 0) || left.id - right.id;
  });
}

export function screenshotDeviceLabel(t: (key: string, options?: any) => string, deviceType?: string) {
  return deviceType === 'MOBILE' ? t('drawer.screenshotDeviceMobile') : t('drawer.screenshotDeviceDesktop');
}
