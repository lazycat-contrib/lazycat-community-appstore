import { ChevronLeft, ChevronRight } from 'lucide-react';
import { useMemo, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { ToggleButton as XToggleButton, ToggleButtonGroup as XToggleButtonGroup } from '@astryxdesign/core/ToggleButton';
import { localizedName } from '../../shared/utils';
import { categoryBrowserState, categoryValue, type BrowserCategory } from './categoryBrowserState';

export function CategoryBrowser({
  categories,
  activeCategory,
  onCategory,
}: {
  categories: BrowserCategory[];
  activeCategory: string;
  onCategory: (category: string) => void;
}) {
  const { t } = useTranslation();
  const childRailRef = useRef<HTMLDivElement | null>(null);
  const { roots, selectedParent, parentValue, railItems } = useMemo(
    () => categoryBrowserState(categories, activeCategory, localizedName),
    [activeCategory, categories],
  );

  function scrollChildren(direction: -1 | 1) {
    childRailRef.current?.scrollBy({ left: direction * 260, behavior: 'smooth' });
  }

  if (categories.length === 0) {
    return null;
  }

  return (
    <div className="category-browser">
      <div className="category-parent-tabs">
        <XToggleButtonGroup
          value={parentValue}
          onChange={(value) => onCategory(value || 'all')}
          label={t('search.categoryFilter')}
          size="sm"
        >
          <XToggleButton value="all" label={t('search.allCategories')} />
          {roots.map((category) => (
            <XToggleButton key={categoryValue(category)} value={categoryValue(category)} label={localizedName(category)} />
          ))}
        </XToggleButtonGroup>
      </div>

      {railItems.length > 0 && (
        <div
          className="category-subrail"
          aria-label={selectedParent ? t('search.subcategoryFilter', { category: localizedName(selectedParent) }) : t('search.categoryFilter')}
        >
          <XIconButton type="button" variant="ghost" label={t('common.previous')} icon={<ChevronLeft size={17} />} onClick={() => scrollChildren(-1)} />
          <div className="category-subrail-scroll" ref={childRailRef}>
            {selectedParent && (
              <XButton
                type="button"
                variant={activeCategory === categoryValue(selectedParent) ? 'primary' : 'secondary'}
                size="sm"
                label={t('search.allInCategory', { category: localizedName(selectedParent) })}
                onClick={() => onCategory(categoryValue(selectedParent))}
              />
            )}
            {railItems.map((item) => (
              <XButton
                type="button"
                key={categoryValue(item.category)}
                variant={activeCategory === categoryValue(item.category) ? 'primary' : 'secondary'}
                size="sm"
                label={item.label}
                onClick={() => onCategory(categoryValue(item.category))}
              />
            ))}
          </div>
          <XIconButton type="button" variant="ghost" label={t('common.next')} icon={<ChevronRight size={17} />} onClick={() => scrollChildren(1)} />
        </div>
      )}
    </div>
  );
}
