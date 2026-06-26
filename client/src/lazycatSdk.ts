type InstallTarget = {
  name: string;
  appId?: string;
  pkgId?: string;
  downloadUrl?: string;
  sha256?: string;
};

type InstallResult = {
  mode: 'lazycat-sdk' | 'download' | 'missing-url' | 'checksum-failed' | 'download-blocked';
  message: string;
};

let gateway: any = null;

async function getGateway() {
  const { lzcAPIGateway } = await import('@lazycatcloud/sdk');
  if (!gateway) {
    gateway = new lzcAPIGateway(window.location.origin, false);
  }
  return gateway;
}

export async function queryInstalledApplications() {
  const gateway = await getGateway();
  return gateway.pkgm.QueryApplication({ deployIds: [] });
}

export async function installWithLazyCat(target: InstallTarget): Promise<InstallResult> {
  if (!target.downloadUrl) {
    return { mode: 'missing-url', message: '没有可下载的 LPK 地址' };
  }

  try {
    const gateway = await getGateway();
    await gateway.pkgm.InstallLPK({
      lpkUrl: target.downloadUrl,
      waitUnitDone: true,
      sha256: target.sha256 || undefined,
      pkgId: target.pkgId || target.appId || undefined,
      tmpTitle: target.name,
    });
    return { mode: 'lazycat-sdk', message: '已通过 LazyCat SDK 安装 LPK' };
  } catch (error) {
    console.warn('[lazycat-sdk] InstallLPK unavailable or install failed, falling back to download', error);
  }

  if (!target.sha256) {
    window.open(target.downloadUrl, '_blank', 'noopener,noreferrer');
    return { mode: 'download', message: '已打开 LPK 下载地址' };
  }

  try {
    const response = await fetch(target.downloadUrl, { credentials: 'include' });
    if (!response.ok) {
      return { mode: 'download-blocked', message: `LPK 下载失败：HTTP ${response.status}` };
    }
    const blob = await response.blob();
    const digest = await sha256Blob(blob);
    if (digest !== target.sha256.toLowerCase()) {
      return { mode: 'checksum-failed', message: 'LPK 文件校验失败，已中止下载' };
    }
    downloadBlob(blob, `${safeFilename(target.name)}.lpk`);
    return { mode: 'download', message: 'LPK 校验通过，已开始浏览器下载' };
  } catch (error) {
    console.warn('[lazycat-sdk] browser fallback download unavailable', error);
    return { mode: 'download-blocked', message: '当前浏览器无法校验该 LPK，请在 LazyCat 客户端内安装或检查源站 CORS 配置' };
  }
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
