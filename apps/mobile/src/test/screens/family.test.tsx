import AsyncStorage from '@react-native-async-storage/async-storage';
import { fireEvent, screen, waitFor } from '@testing-library/react-native';
import { router } from 'expo-router';

import { resolveApiClient } from '../../api/resolveClient';
import FamilyScreen from '../../app/(tabs)/index';
import { CURRENT_MEMBER_STORAGE_KEY } from '../../members/currentMemberStore';
import { createFakeApiClient, TEST_BASE_URL } from '../fakeApi';
import {
  makeFlagCounts,
  makeMember,
  makePanelSummary,
  makeReportSummary,
} from '../fixtures';
import { renderScreen } from '../renderScreen';

jest.mock('../../api/resolveClient', () => ({
  resolveApiClient: jest.fn(),
}));

const mockedResolveApiClient = jest.mocked(resolveApiClient);

function useApi(routes: Parameters<typeof createFakeApiClient>[0]) {
  mockedResolveApiClient.mockResolvedValue({
    client: createFakeApiClient(routes),
    baseUrl: TEST_BASE_URL,
  });
}

beforeEach(async () => {
  jest.clearAllMocks();
  await AsyncStorage.clear();
});

describe('FamilyScreen', () => {
  it('renders pending/abnormal summaries and opens the member archive', async () => {
    useApi({
      GET: {
        '/members': () => ({
          members: [
            makeMember({ member_id: 1, name: '小明', is_child: true }),
            makeMember({ member_id: 2, name: '妈妈', is_child: false }),
          ],
        }),
        '/reports': ({ params } = {}) => ({
          reports:
            params?.query?.member_id === 1
              ? [
                  makeReportSummary({
                    status: 'partial',
                    unreviewed_low_confidence_count: 1,
                    flag_counts: makeFlagCounts({ high: 1 }),
                  }),
                ]
              : [],
        }),
        '/members/{member_id}/panels': ({ params } = {}) => ({
          panels:
            params?.path?.member_id === 1
              ? [makePanelSummary({ abnormal_count: 1, item_count: 2 })]
              : [],
        }),
      },
    });

    await renderScreen(<FamilyScreen />);

    expect(await screen.findByText('小明')).toBeTruthy();
    expect(screen.getByText('孩子')).toBeTruthy();
    expect(screen.getByTestId('family-member-pending-1')).toHaveTextContent(
      '待核对 1 份 · 1 项',
    );
    expect(screen.getByTestId('family-member-abnormal-1')).toHaveTextContent(
      '异常指标 1 项 · 1 个项目',
    );
    expect(screen.getByTestId('family-member-latest-1')).toHaveTextContent(/最近：肝功/);

    await fireEvent.press(screen.getByTestId('family-member-1'));

    expect(router.push).toHaveBeenCalledWith('/panels');
    await waitFor(() =>
      expect(AsyncStorage.setItem).toHaveBeenCalledWith(CURRENT_MEMBER_STORAGE_KEY, '1'),
    );
  });

  it('shows the backend error and retries the load', async () => {
    let failing = true;
    useApi({
      GET: {
        '/members': () => {
          if (failing) {
            throw new Error('backend down');
          }
          return { members: [makeMember({ member_id: 1, name: '小明' })] };
        },
        '/reports': () => ({ reports: [] }),
        '/members/{member_id}/panels': () => ({ panels: [] }),
      },
    });

    await renderScreen(<FamilyScreen />);

    expect(await screen.findByText('加载失败:backend down')).toBeTruthy();

    failing = false;
    await fireEvent.press(screen.getByText('重试'));

    expect(await screen.findByText('小明')).toBeTruthy();
  });

  it('shows the empty-family guidance when there are no members', async () => {
    useApi({
      GET: {
        '/members': () => ({ members: [] }),
      },
    });

    await renderScreen(<FamilyScreen />);

    expect(await screen.findByText(/还没有家庭成员/)).toBeTruthy();
  });
});
