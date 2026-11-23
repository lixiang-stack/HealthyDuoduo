import AsyncStorage from '@react-native-async-storage/async-storage';

import { DEFAULT_DOMAIN_NAVIGATION } from './domainRoutes';
import {
  DOMAIN_NAVIGATION_STORAGE_KEY,
  loadDomainNavigation,
  saveDomainNavigation,
} from './domainNavigationStore';

beforeEach(async () => {
  await AsyncStorage.clear();
});

describe('domain navigation store', () => {
  it('returns the default entry when nothing is stored', async () => {
    expect(await loadDomainNavigation()).toEqual(DEFAULT_DOMAIN_NAVIGATION);
  });

  it('round-trips the remembered domain and secondary views', async () => {
    const state = {
      lastDomain: 'health' as const,
      lastPaths: { daily: '/daily/stats' as const, health: '/health/trends' as const },
    };

    await saveDomainNavigation(state);

    expect(await loadDomainNavigation()).toEqual(state);
  });

  it('repairs malformed or partial stored state', async () => {
    await AsyncStorage.setItem(
      DOMAIN_NAVIGATION_STORAGE_KEY,
      JSON.stringify({ lastDomain: 'nope', lastPaths: { daily: '/daily/nope' } }),
    );

    expect(await loadDomainNavigation()).toEqual(DEFAULT_DOMAIN_NAVIGATION);
  });

  it('falls back to defaults for broken JSON', async () => {
    await AsyncStorage.setItem(DOMAIN_NAVIGATION_STORAGE_KEY, '{not json');

    expect(await loadDomainNavigation()).toEqual(DEFAULT_DOMAIN_NAVIGATION);
  });
});
