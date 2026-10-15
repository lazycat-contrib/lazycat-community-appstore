import { ChevronDown, ChevronUp, Search, X } from 'lucide-react';
import { useMemo, useState, type KeyboardEvent } from 'react';
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
  const [query, setQuery] = useState('');
  const [expandedParent, setExpandedParent] = useState('');
  const { roots, selectedParent, parentValue, railItems, searchItems } = useMemo(
    () => categoryBrowserState(categories, activeCategory, localizedName),
    [activeCategory, categories],
  );
  const showSearch = categories.length > 16;
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const searchMatches = useMemo(
    () => normalizedQuery ? searchItems.filter((item) => item.label.toLocaleLowerCase().includes(normalizedQuery)) : [],
    [normalizedQuery, searchItems],
  );
  const activeChildIndex = railItems.findIndex((item) => categoryValue(item.category) === activeCategory);
  const expanded = expandedParent === parentValue || activeChildIndex >= 12;
  const visibleRailItems = expanded ? railItems : railItems.slice(0, 12);

  function chooseCategory(value: string) {
    setQuery('');
    setExpandedParent('');
    onCategory(value);
  }

  function onSearchKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Escape') setQuery('');
    if (event.key === 'Enter' && searchMatches[0]) {
      event.preventDefault();
      chooseCategory(categoryValue(searchMatches[0].category));
    }
  }

  if (categories.length === 0) return null;

  return (
    <div className="category-browser">
      {showSearch && (
        <div className="category-browser-toolbar">
          <div className="category-browser-heading">
            <strong>{t('search.categoryFilter')}</strong>
            <span>{t('search.categoryBrowseHint')}</span>
          </div>
          <div className="category-search-field">
            <Search size={17} aria-hidden="true" />
            <input
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              onKeyDown={onSearchKeyDown}
              placeholder={t('search.categorySearchPlaceholder')}
              aria-label={t('search.categorySearchLabel')}
            />
            {query && (
              <XIconButton
                type="button"
                variant="ghost"
                size="sm"
                label={t('search.clearCategorySearch')}
                icon={<X size={16} />}
                onClick={() => setQuery('')}
              />
            )}
          </div>
        </div>
      )}

      {normalizedQuery ? (
        <div className="category-search-panel" role="group" aria-label={t('search.categorySearchLabel')}>
          <p role="status">{t('search.categorySearchResults', { count: searchMatches.length })}</p>
          {searchMatches.length > 0 ? (
            <div className="category-search-results">
              {searchMatches.slice(0, 12).map((item) => (
                <button
                  type="button"
                  className="category-search-result"
                  key={categoryValue(item.category)}
                  aria-pressed={activeCategory === categoryValue(item.category)}
                  onClick={() => chooseCategory(categoryValue(item.category))}
                >
                  {item.label}
                </button>
              ))}
            </div>
          ) : <p>{t('search.categorySearchEmpty')}</p>}
          {searchMatches.length > 12 && <p>{t('search.categorySearchMore')}</p>}
        </div>
      ) : (
        <>
          <div className="category-parent-tabs">
            <XToggleButtonGroup
              value={parentValue}
              onChange={(value) => chooseCategory(value || 'all')}
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
              role="group"
              aria-label={selectedParent ? t('search.subcategoryFilter', { category: localizedName(selectedParent) }) : t('search.categoryFilter')}
            >
              <div className="category-subrail-items">
                {selectedParent && (
                  <XButton
                    type="button"
                    variant={activeCategory === categoryValue(selectedParent) ? 'primary' : 'secondary'}
                    size="sm"
                    label={t('search.allInCategory', { category: localizedName(selectedParent) })}
                    onClick={() => chooseCategory(categoryValue(selectedParent))}
                  />
                )}
                {visibleRailItems.map((item, index) => (
                  <XButton
                    type="button"
                    key={categoryValue(item.category)}
                    className={index >= 12 ? 'category-subrail-extra' : undefined}
                    variant={activeCategory === categoryValue(item.category) ? 'primary' : 'secondary'}
                    size="sm"
                    label={item.label}
                    onClick={() => chooseCategory(categoryValue(item.category))}
                  />
                ))}
              </div>
              {railItems.length > 12 && activeChildIndex < 12 && (
                <XButton
                  type="button"
                  variant="ghost"
                  size="sm"
                  label={expanded ? t('search.showFewerCategories') : t('search.showMoreCategories', { count: railItems.length - 12 })}
                  icon={expanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
                  onClick={() => setExpandedParent(expanded ? '' : parentValue)}
                />
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
}
