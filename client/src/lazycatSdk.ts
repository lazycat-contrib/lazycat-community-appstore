export type InstalledApplication = {
  appid?: string;
  title?: string;
  version?: string;
  status?: number;
};

export type InstalledApplicationsResponse = {
  infoList?: InstalledApplication[];
};

type InstallTarget = {
  name: string;
  appId?: string;
  pkgId?: string;
  downloadUrl?: string;
  sha256?: string;
};

type InstallResult = {
  mode: 'lazycat-sdk' | 'download' | 'missing-url' | 'checksum-failed' | 'download-blocked';
  messageKey: string;
  messageParams?: Record<string, string | number>;
};

let gateway: any = null;
const INSTALL_HANDOFF_TIMEOUT_MS = 6000;

async function getGateway() {
  const { lzcAPIGateway } = await import('@lazycatcloud/sdk');
  if (!gateway) {
    gateway = new lzcAPIGateway(window.location.origin, false);
  }
  return gateway;
}

export async function queryInstalledApplications(): Promise<InstalledApplicationsResponse> {
  const gateway = await getGateway();
  return gateway.pkgm.QueryApplication({ appidList: [] });
}

export async function installWithLazyCat(target: InstallTarget): Promise<InstallResult> {
  if (!target.downloadUrl) {
    return { mode: 'missing-url', messageKey: 'installResult.missingUrl' };
  }

  try {
    const gateway = await getGateway();
    await withTimeout(
      gateway.pkgm.InstallLPK({
        lpkUrl: target.downloadUrl,
        waitUnitDone: true,
        sha256: target.sha256 || undefined,
        pkgId: target.pkgId || target.appId || undefined,
        tmpTitle: target.name,
      }),
      INSTALL_HANDOFF_TIMEOUT_MS,
    );
    return { mode: 'lazycat-sdk', messageKey: 'installResult.sdkInstalled' };
  } catch (error) {
    console.warn('[lazycat-sdk] InstallLPK unavailable or install failed, falling back to download', error);
  }

  if (!target.sha256) {
    window.open(target.downloadUrl, '_blank', 'noopener,noreferrer');
    return { mode: 'download', messageKey: 'installResult.downloadOpened' };
  }

  try {
    const response = await fetch(target.downloadUrl, { credentials: 'include' });
    if (!response.ok) {
      return { mode: 'download-blocked', messageKey: 'installResult.downloadFailedHttp', messageParams: { status: response.status } };
    }
    const blob = await response.blob();
    const digest = await sha256Blob(blob);
    if (digest !== target.sha256.toLowerCase()) {
      return { mode: 'checksum-failed', messageKey: 'installResult.checksumFailed' };
    }
    downloadBlob(blob, `${safeFilename(target.name)}.lpk`);
    return { mode: 'download', messageKey: 'installResult.checksumPassed' };
  } catch (error) {
    console.warn('[lazycat-sdk] browser fallback download unavailable', error);
    return { mode: 'download-blocked', messageKey: 'installResult.browserVerifyUnavailable' };
  }
}

function withTimeout<T>(promise: PromiseLike<T> | T, timeoutMs: number): Promise<T> {
  return new Promise((resolve, reject) => {
    const timeout = window.setTimeout(() => reject(new Error('LazyCat SDK install timed out')), timeoutMs);
    Promise.resolve(promise).then(
      (value) => {
        window.clearTimeout(timeout);
        resolve(value);
      },
      (error) => {
        window.clearTimeout(timeout);
        reject(error);
      },
    );
  });
}

async function sha256Blob(blob: Blob) {
  const buffer = await blob.arrayBuffer();
  const hash = await crypto.subtle.digest('SHA-256', buffer);
  return Array.from(new Uint8Array(hash))
    .map((byte) => byte.toString(16).padStart(2, '0'))
    .join('');
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}

function safeFilename(value: string) {
  const safe = value.trim().replace(/[^a-zA-Z0-9._-]+/g, '-').replace(/^-+|-+$/g, '');
  return safe || 'app';
}
