import { ChevronLeft, ChevronRight } from 'lucide-react';
import { useMemo, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { ToggleButton as XToggleButton, ToggleButtonGroup as XToggleButtonGroup } from '@astryxdesign/core/ToggleButton';
import { buildCategoryHierarchy } from '../../shared/categoryTree';
import type { Category } from '../../shared/types';
import { localizedName } from '../../shared/utils';

export type BrowserCategory = Category & { value?: string };

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
  const hierarchy = useMemo(() => buildCategoryHierarchy(categories), [categories]);
  const activeRecord = activeCategory === 'all' ? undefined : categories.find((category) => categoryValue(category) === activeCategory);
  const selectedParentID = activeRecord?.parentId || activeRecord?.id || null;
  const selectedParent = selectedParentID ? hierarchy.byID.get(selectedParentID) : undefined;
  const childCategories = selectedParent ? hierarchy.childrenByParent.get(selectedParent.id) || [] : [];
  const parentValue = selectedParent ? categoryValue(selectedParent) : 'all';

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
          {hierarchy.roots.map((category) => (
            <XToggleButton key={categoryValue(category)} value={categoryValue(category)} label={localizedName(category)} />
          ))}
        </XToggleButtonGroup>
      </div>

      {selectedParent && childCategories.length > 0 && (
        <div className="category-subrail" aria-label={t('search.subcategoryFilter', { category: localizedName(selectedParent) })}>
          <XIconButton type="button" variant="ghost" label={t('common.previous')} icon={<ChevronLeft size={17} />} onClick={() => scrollChildren(-1)} />
          <div className="category-subrail-scroll" ref={childRailRef}>
            <XButton
              type="button"
              variant={activeCategory === categoryValue(selectedParent) ? 'primary' : 'secondary'}
              size="sm"
              label={t('search.allInCategory', { category: localizedName(selectedParent) })}
              onClick={() => onCategory(categoryValue(selectedParent))}
            />
            {childCategories.map((category) => (
              <XButton
                type="button"
                key={categoryValue(category)}
                variant={activeCategory === categoryValue(category) ? 'primary' : 'secondary'}
                size="sm"
                label={localizedName(category)}
                onClick={() => onCategory(categoryValue(category))}
              />
            ))}
          </div>
          <XIconButton type="button" variant="ghost" label={t('common.next')} icon={<ChevronRight size={17} />} onClick={() => scrollChildren(1)} />
        </div>
      )}
    </div>
  );
}

function categoryValue(category: Category) {
  return (category as BrowserCategory).value || String(category.id);
}
