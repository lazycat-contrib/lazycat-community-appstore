import type { DefinedTheme } from '@astryxdesign/core/theme';
import { butterTheme } from '@astryxdesign/theme-butter/built';
import { chocolateTheme } from '@astryxdesign/theme-chocolate/built';
import { gothicTheme } from '@astryxdesign/theme-gothic/built';
import { matchaTheme } from '@astryxdesign/theme-matcha/built';
import { neutralTheme } from '@astryxdesign/theme-neutral/built';
import { stoneTheme } from '@astryxdesign/theme-stone/built';
import { y2kTheme } from '@astryxdesign/theme-y2k/built';

export const ASTRYX_THEMES = [
  { name: 'neutral', theme: neutralTheme, labelKey: 'theme.names.neutral' },
  { name: 'butter', theme: butterTheme, labelKey: 'theme.names.butter' },
  { name: 'matcha', theme: matchaTheme, labelKey: 'theme.names.matcha' },
  { name: 'stone', theme: stoneTheme, labelKey: 'theme.names.stone' },
  { name: 'gothic', theme: gothicTheme, labelKey: 'theme.names.gothic' },
  { name: 'chocolate', theme: chocolateTheme, labelKey: 'theme.names.chocolate' },
  { name: 'y2k', theme: y2kTheme, labelKey: 'theme.names.y2k' },
] as const;

export type AstryxThemeName = (typeof ASTRYX_THEMES)[number]['name'];

export type AstryxThemeEntry = {
  name: AstryxThemeName;
  theme: DefinedTheme;
  labelKey: string;
};

export function getAstryxTheme(name?: string | null): AstryxThemeEntry {
  return ASTRYX_THEMES.find((theme) => theme.name === name) || ASTRYX_THEMES[0];
}
