import AsyncStorage from '@react-native-async-storage/async-storage';
import { render, screen } from '@testing-library/react-native';

import { resolveApiClient } from '../../api/resolveClient';
import IndexScreen from '../../app/index';
import { DEFAULT_DOMAIN_NAVIGATION } from '../../navigation/domainRoutes';
import { DOMAIN_NAVIGATION_STORAGE_KEY } from '../../navigation/domainNavigationStore';
import { createFakeApiClient, TEST_BASE_URL } from '../fakeApi';
import { makeMember } from '../fixtures';

jest.mock('../../api/resolveClient', () => ({
  resolveApiClient: jest.fn(),
}));

const mockedResolveApiClient = jest.mocked(resolveApiClient);

function useApi(members: ReturnType<typeof makeMember>[]) {
  mockedResolveApiClient.mockResolvedValue({
    client: createFakeApiClient({ GET: { '/members': () => ({ members }) } }),
    baseUrl: TEST_BASE_URL,
  });
}

beforeEach(async () => {
  jest.clearAllMocks();
  await AsyncStorage.clear();
});

describe('entry route', () => {
  it('starts in care today when the family has a child', async () => {
    useApi([makeMember({ member_id: 1, is_child: true })]);

    await render(<IndexScreen />);

    expect(await screen.findByTestId('redirect')).toHaveTextContent('/care/today');
  });

  it('starts in health reports when the family has no child', async () => {
    useApi([makeMember({ member_id: 2, name: '妈妈', is_child: false })]);

    await render(<IndexScreen />);

    expect(await screen.findByTestId('redirect')).toHaveTextContent('/health/reports');
  });

  it('restores the last domain and secondary view without reloading members', async () => {
    await AsyncStorage.setItem(
      DOMAIN_NAVIGATION_STORAGE_KEY,
      JSON.stringify({
        lastDomain: 'health',
        lastPaths: { care: '/care/stats', health: '/health/trends' },
      }),
    );

    await render(<IndexScreen />);

    expect(await screen.findByTestId('redirect')).toHaveTextContent('/health/trends');
    expect(mockedResolveApiClient).not.toHaveBeenCalled();
  });

  it('falls back to health reports when the member list cannot load', async () => {
    mockedResolveApiClient.mockResolvedValue(undefined);

    await render(<IndexScreen />);

    expect(await screen.findByTestId('redirect')).toHaveTextContent('/health/reports');
    expect(DEFAULT_DOMAIN_NAVIGATION.lastPaths.health).toBe('/health/reports');
  });
});
