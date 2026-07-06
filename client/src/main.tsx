import React from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import './i18n';
import '@astryxdesign/core/reset.css';
import '@astryxdesign/core/astryx.css';
import '@astryxdesign/theme-butter/theme.css';
import '@astryxdesign/theme-chocolate/theme.css';
import '@astryxdesign/theme-gothic/theme.css';
import '@astryxdesign/theme-matcha/theme.css';
import '@astryxdesign/theme-neutral/theme.css';
import '@astryxdesign/theme-stone/theme.css';
import '@astryxdesign/theme-y2k/theme.css';
import './styles.css';

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
