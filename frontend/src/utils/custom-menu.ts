import type { CustomMenuItem, CustomMenuPlacement } from '@/types'

export const DEFAULT_CUSTOM_MENU_PLACEMENT: CustomMenuPlacement = 'sidebar'

export function normalizeCustomMenuPlacement(
  value: unknown,
): CustomMenuPlacement {
  if (value === 'home_header' || value === 'both') {
    return value
  }
  return DEFAULT_CUSTOM_MENU_PLACEMENT
}

export function normalizeCustomMenuItem(item: CustomMenuItem): CustomMenuItem {
  return {
    ...item,
    placement: normalizeCustomMenuPlacement(item.placement),
  }
}

export function normalizeCustomMenuItems(
  items: CustomMenuItem[] | null | undefined,
): CustomMenuItem[] {
  if (!Array.isArray(items)) {
    return []
  }
  return items.map((item) => normalizeCustomMenuItem(item))
}

export function isSidebarMenuPlacement(
  item: Pick<CustomMenuItem, 'placement'>,
): boolean {
  const placement = normalizeCustomMenuPlacement(item.placement)
  return placement === 'sidebar' || placement === 'both'
}

export function isHomeHeaderMenuPlacement(
  item: Pick<CustomMenuItem, 'placement'>,
): boolean {
  const placement = normalizeCustomMenuPlacement(item.placement)
  return placement === 'home_header' || placement === 'both'
}

export function getCustomMenuRoute(id: string): string {
  return `/custom/${encodeURIComponent(id)}`
}

export function getHomeHeaderMenuHref(
  item: Pick<CustomMenuItem, 'id' | 'url' | 'page_slug'>,
): string {
  if (item.page_slug || item.url?.startsWith('md:')) {
    return getCustomMenuRoute(item.id)
  }

  const rawUrl = item.url?.trim()
  if (rawUrl) {
    return rawUrl
  }

  return getCustomMenuRoute(item.id)
}
