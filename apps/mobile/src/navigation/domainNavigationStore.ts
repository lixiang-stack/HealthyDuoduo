// Device-local navigation preference: which domain was used last and which secondary
// view each domain should resume. It never leaves the device (no accounts / sync).
import AsyncStorage from '@react-native-async-storage/async-storage';

import {
  DEFAULT_DOMAIN_NAVIGATION,
  isDailyPath,
  isHealthPath,
  type DomainNavigationState,
} from './domainRoutes';

export const DOMAIN_NAVIGATION_STORAGE_KEY = 'healthyduoduo.domainNavigation';

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
    const daily = isDailyPath(paths.daily)
      ? paths.daily
      : DEFAULT_DOMAIN_NAVIGATION.lastPaths.daily;
    const health = isHealthPath(paths.health)
      ? paths.health
      : DEFAULT_DOMAIN_NAVIGATION.lastPaths.health;
    const lastDomain =
      record.lastDomain === 'daily' || record.lastDomain === 'health'
        ? record.lastDomain
        : undefined;
    return { lastDomain, lastPaths: { daily, health } };
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
