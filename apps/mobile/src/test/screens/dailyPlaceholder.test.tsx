import AsyncStorage from '@react-native-async-storage/async-storage';
import { screen } from '@testing-library/react-native';

import { resolveApiClient } from '../../api/resolveClient';
import { DailyPlaceholder } from '../../daily/DailyPlaceholder';
import { CURRENT_MEMBER_STORAGE_KEY } from '../../members/currentMemberStore';
import { createFakeApiClient, TEST_BASE_URL } from '../fakeApi';
import { makeMember } from '../fixtures';
import { renderScreen } from '../renderScreen';

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

describe('DailyPlaceholder', () => {
  it('guides away from an adult current member', async () => {
    await AsyncStorage.setItem(CURRENT_MEMBER_STORAGE_KEY, '2');
    useApi([
      makeMember({ member_id: 1, name: '豆豆', is_child: true }),
      makeMember({ member_id: 2, name: '妈妈', is_child: false }),
    ]);

    await renderScreen(<DailyPlaceholder variant="today" />);

    expect(await screen.findByTestId('daily-adult-guidance')).toHaveTextContent(/妈妈/);
    expect(screen.getByTestId('daily-adult-guidance')).toHaveTextContent(/孩子/);
  });

  it('asks for a child when the family has none', async () => {
    await AsyncStorage.setItem(CURRENT_MEMBER_STORAGE_KEY, '2');
    useApi([makeMember({ member_id: 2, name: '妈妈', is_child: false })]);

    await renderScreen(<DailyPlaceholder variant="today" />);

    expect(await screen.findByTestId('daily-no-child')).toHaveTextContent(/还没有孩子成员/);
  });

  it('renders the today and stats placeholders for a child context', async () => {
    await AsyncStorage.setItem(CURRENT_MEMBER_STORAGE_KEY, '1');
    useApi([makeMember({ member_id: 1, name: '豆豆', is_child: true })]);

    const today = await renderScreen(<DailyPlaceholder variant="today" />);
    expect(await screen.findByTestId('daily-today-placeholder')).toBeTruthy();
    await today.unmount();

    await renderScreen(<DailyPlaceholder variant="stats" />);
    expect(await screen.findByTestId('daily-stats-placeholder')).toBeTruthy();
    expect(screen.getByText(/4 个指标/)).toBeTruthy();
  });
});
