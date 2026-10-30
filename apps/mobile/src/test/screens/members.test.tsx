import AsyncStorage from '@react-native-async-storage/async-storage';
import { fireEvent, screen, waitFor } from '@testing-library/react-native';

import { resolveApiClient } from '../../api/resolveClient';
import MembersScreen from '../../app/members';
import { CURRENT_MEMBER_STORAGE_KEY } from '../../members/currentMemberStore';
import { createFakeApiClient, TEST_BASE_URL, type FakeApiOptions } from '../fakeApi';
import { makeMember, makeReportSummary } from '../fixtures';
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

describe('MembersScreen', () => {
  it('deletes a member with reports after showing the irreversible count and clears the current selection', async () => {
    let members = [
      makeMember({ member_id: 1, name: '小明', is_child: true }),
      makeMember({ member_id: 2, name: '妈妈', is_child: false }),
    ];
    const deleteHandler = jest.fn((options?: FakeApiOptions) => {
      const id = Number(options?.params?.path?.member_id);
      members = members.filter((member) => member.member_id !== id);
      return undefined;
    });
    useApi({
      GET: {
        '/members': () => ({ members }),
        '/reports': ({ params } = {}) => ({
          reports:
            params?.query?.member_id === 1
              ? [makeReportSummary({ report_id: 'r1' }), makeReportSummary({ report_id: 'r2' })]
              : [],
        }),
      },
      DELETE: { '/members/{member_id}': deleteHandler },
    });
    await AsyncStorage.setItem(CURRENT_MEMBER_STORAGE_KEY, '1');

    await renderScreen(<MembersScreen />);
    expect(await screen.findByText('小明')).toBeTruthy();

    fireEvent.press(screen.getByTestId('member-delete-1'));

    expect(await screen.findByText(/及其 2 份报告/)).toBeTruthy();
    expect(screen.getByText(/此操作不可恢复/)).toBeTruthy();

    fireEvent.press(screen.getByTestId('member-delete-confirm-1'));

    await waitFor(() => expect(screen.queryByText('小明')).toBeNull());
    expect(screen.getByText('妈妈')).toBeTruthy();
    expect(deleteHandler).toHaveBeenCalledTimes(1);
    await waitFor(() =>
      expect(AsyncStorage.removeItem).toHaveBeenCalledWith(CURRENT_MEMBER_STORAGE_KEY),
    );
  });

  it('shows the generic irreversible confirmation when the member has no reports', async () => {
    useApi({
      GET: {
        '/members': () => ({ members: [makeMember({ member_id: 1, name: '小明' })] }),
        '/reports': () => ({ reports: [] }),
      },
      DELETE: { '/members/{member_id}': jest.fn() },
    });

    await renderScreen(<MembersScreen />);
    expect(await screen.findByText('小明')).toBeTruthy();

    fireEvent.press(screen.getByTestId('member-delete-1'));

    expect(await screen.findByText('删除成员「小明」？此操作不可恢复。')).toBeTruthy();
    expect(screen.getByTestId('member-delete-confirm-1')).toBeTruthy();
  });

  it('blocks deletion when the report count cannot be loaded', async () => {
    const deleteHandler = jest.fn();
    useApi({
      GET: {
        '/members': () => ({ members: [makeMember({ member_id: 1, name: '小明' })] }),
        '/reports': () => {
          throw new Error('backend down');
        },
      },
      DELETE: { '/members/{member_id}': deleteHandler },
    });

    await renderScreen(<MembersScreen />);
    expect(await screen.findByText('小明')).toBeTruthy();

    fireEvent.press(screen.getByTestId('member-delete-1'));

    expect(await screen.findByText('读取报告失败:backend down')).toBeTruthy();
    expect(screen.queryByTestId('member-delete-confirm-1')).toBeNull();
    expect(deleteHandler).not.toHaveBeenCalled();

    fireEvent.press(screen.getByTestId('member-delete-retry-1'));
    expect(await screen.findByText('读取报告失败:backend down')).toBeTruthy();
    expect(screen.queryByTestId('member-delete-confirm-1')).toBeNull();
  });
});
