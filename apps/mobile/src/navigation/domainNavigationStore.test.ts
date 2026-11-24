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
      lastPaths: { care: '/care/stats' as const, health: '/health/trends' as const },
    };

    await saveDomainNavigation(state);

    expect(await loadDomainNavigation()).toEqual(state);
  });

  it('migrates the pre-rename daily preference to care', async () => {
    await AsyncStorage.setItem(
      DOMAIN_NAVIGATION_STORAGE_KEY,
      JSON.stringify({
        lastDomain: 'daily',
        lastPaths: { daily: '/daily/stats', health: '/health/trends' },
      }),
    );

    expect(await loadDomainNavigation()).toEqual({
      lastDomain: 'care',
      lastPaths: { care: '/care/stats', health: '/health/trends' },
    });
  });

  it('repairs malformed or partial stored state', async () => {
    await AsyncStorage.setItem(
      DOMAIN_NAVIGATION_STORAGE_KEY,
      JSON.stringify({ lastDomain: 'nope', lastPaths: { care: '/care/nope' } }),
    );

    expect(await loadDomainNavigation()).toEqual(DEFAULT_DOMAIN_NAVIGATION);
  });

  it('falls back to defaults for broken JSON', async () => {
    await AsyncStorage.setItem(DOMAIN_NAVIGATION_STORAGE_KEY, '{not json');

    expect(await loadDomainNavigation()).toEqual(DEFAULT_DOMAIN_NAVIGATION);
  });
});
