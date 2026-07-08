type RuntimeConfig = {
  apiBaseURL?: string;
  defaultSourceURL?: string;
  defaultSourceName?: string;
  appVersion?: string;
};

declare global {
  interface Window {
    LAZYCAT_APPSTORE_CONFIG?: RuntimeConfig;
  }
}

function cleanURL(value?: string) {
  return (value || '').trim().replace(/\/+$/, '');
}

const runtimeConfig = window.LAZYCAT_APPSTORE_CONFIG || {};

export const API_BASE = cleanURL(runtimeConfig.apiBaseURL || import.meta.env.VITE_API_BASE_URL);
export const HAS_API = API_BASE !== '';
export const DEFAULT_SOURCE_URL = cleanURL(runtimeConfig.defaultSourceURL);
export const DEFAULT_SOURCE_NAME = (runtimeConfig.defaultSourceName || '懒猫私有商店').trim() || '懒猫私有商店';
export const APP_VERSION = (runtimeConfig.appVersion || import.meta.env.VITE_APP_VERSION || 'dev').trim() || 'dev';
