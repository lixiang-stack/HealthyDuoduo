// Device-local navigation preference: which domain was used last and which secondary
// view each domain should resume. It never leaves the device (no accounts / sync).
import AsyncStorage from '@react-native-async-storage/async-storage';

import {
  DEFAULT_DOMAIN_NAVIGATION,
  isCarePath,
  isHealthPath,
  type CarePath,
  type Domain,
  type DomainNavigationState,
} from './domainRoutes';

export const DOMAIN_NAVIGATION_STORAGE_KEY = 'healthyduoduo.domainNavigation';

// The care domain used to live at /daily. Keep the old preference across the rename
// instead of resetting the user's last domain and secondary tab.
const LEGACY_DAILY_PATHS = {
  '/daily/today': '/care/today',
  '/daily/stats': '/care/stats',
} as const;

type LegacyDailyPath = keyof typeof LEGACY_DAILY_PATHS;

function isLegacyDailyPath(value: unknown): value is LegacyDailyPath {
  return (
    typeof value === 'string' &&
    Object.prototype.hasOwnProperty.call(LEGACY_DAILY_PATHS, value)
  );
}

function storedCarePath(value: unknown): CarePath {
  if (isCarePath(value)) {
    return value;
  }
  if (isLegacyDailyPath(value)) {
    return LEGACY_DAILY_PATHS[value];
  }
  return DEFAULT_DOMAIN_NAVIGATION.lastPaths.care;
}

function storedDomain(value: unknown): Domain | undefined {
  if (value === 'care' || value === 'health') {
    return value;
  }
  if (value === 'daily') {
    return 'care';
  }
  return undefined;
}

function parseStored(raw: string): DomainNavigationState {
  try {
    const value: unknown = JSON.parse(raw);
    if (value === null || typeof value !== 'object') {
      return DEFAULT_DOMAIN_NAVIGATION;
    }
    const record = value as Record<string, unknown>;
    const paths =
      record.lastPaths !== null && typeof record.lastPaths === 'object'
        ? (record.lastPaths as Record<string, unknown>)
        : {};
    const care = storedCarePath(paths.care ?? paths.daily);
    const health = isHealthPath(paths.health)
      ? paths.health
      : DEFAULT_DOMAIN_NAVIGATION.lastPaths.health;
    const lastDomain = storedDomain(record.lastDomain);
    return { lastDomain, lastPaths: { care, health } };
  } catch {
    return DEFAULT_DOMAIN_NAVIGATION;
  }
}

export async function loadDomainNavigation(): Promise<DomainNavigationState> {
  const stored = await AsyncStorage.getItem(DOMAIN_NAVIGATION_STORAGE_KEY);
  return stored ? parseStored(stored) : DEFAULT_DOMAIN_NAVIGATION;
}

export async function saveDomainNavigation(state: DomainNavigationState): Promise<void> {
  await AsyncStorage.setItem(DOMAIN_NAVIGATION_STORAGE_KEY, JSON.stringify(state));
}
