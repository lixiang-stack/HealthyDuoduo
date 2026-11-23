import AsyncStorage from '@react-native-async-storage/async-storage';
import { fireEvent, screen, waitFor } from '@testing-library/react-native';

import { resolveApiClient } from '../../api/resolveClient';
import { DailyPlaceholder } from '../../daily/DailyPlaceholder';
import {
  CURRENT_CHILD_STORAGE_KEY,
  CURRENT_MEMBER_STORAGE_KEY,
} from '../../members/currentMemberStore';
import { mockRouter } from '../routerMock';
import { createFakeApiClient, TEST_BASE_URL } from '../fakeApi';
import { makeMember } from '../fixtures';
import { renderScreen } from '../renderScreen';

jest.mock('../../api/resolveClient', () => ({
  resolveApiClient: jest.fn(),
}));

const mockedResolveApiClient = jest.mocked(resolveApiClient);

const doudou = makeMember({ member_id: 1, name: '豆豆', is_child: true });
const second = makeMember({ member_id: 2, name: '老二', is_child: true });
const third = makeMember({ member_id: 3, name: '老三', is_child: true });
const mom = makeMember({ member_id: 9, name: '妈妈', is_child: false });

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
  it('locks the only child, ignores the adult current member, and persists the selection', async () => {
    await AsyncStorage.setItem(CURRENT_MEMBER_STORAGE_KEY, String(mom.member_id));
    useApi([doudou, mom]);

    await renderScreen(<DailyPlaceholder variant="today" />);

    expect(await screen.findByTestId('daily-current-child')).toHaveTextContent('豆豆');
    expect(screen.queryByTestId('daily-child-switcher')).toBeNull();
    expect(screen.queryByTestId('child-picker')).toBeNull();
    expect(await screen.findByTestId('daily-today-empty')).toHaveTextContent(/记录给 豆豆/);
    await waitFor(() =>
      expect(AsyncStorage.setItem).toHaveBeenCalledWith(CURRENT_CHILD_STORAGE_KEY, '1'),
    );
  });

  it('asks for a child when the family has none and clears a stale selection', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, '1');
    useApi([mom]);

    await renderScreen(<DailyPlaceholder variant="today" />);

    expect(await screen.findByTestId('daily-no-child')).toHaveTextContent(/还没有孩子成员/);
    expect(screen.queryByTestId('daily-child-switcher')).toBeNull();
    await waitFor(() =>
      expect(AsyncStorage.removeItem).toHaveBeenCalledWith(CURRENT_CHILD_STORAGE_KEY),
    );
  });

  it('requires a choice on first entry with several children and persists the pick', async () => {
    useApi([doudou, second, mom]);

    await renderScreen(<DailyPlaceholder variant="today" />);

    expect(await screen.findByTestId('child-picker')).toBeTruthy();
    expect(screen.getByTestId('child-option-1')).toBeTruthy();
    expect(screen.getByTestId('child-option-2')).toBeTruthy();
    expect(screen.queryByTestId('child-option-9')).toBeNull();
    expect(screen.getByTestId('daily-needs-child')).toBeTruthy();

    await fireEvent.press(screen.getByTestId('child-option-2'));

    await waitFor(() => expect(screen.queryByTestId('child-picker')).toBeNull());
    expect(screen.getByTestId('daily-child-switcher')).toHaveTextContent('老二 ▾');
    expect(await screen.findByTestId('daily-today-empty')).toHaveTextContent(/记录给 老二/);
    await waitFor(() =>
      expect(AsyncStorage.setItem).toHaveBeenCalledWith(CURRENT_CHILD_STORAGE_KEY, '2'),
    );
  });

  it('restores a persisted child without reopening the picker', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(second.member_id));
    useApi([doudou, second]);

    await renderScreen(<DailyPlaceholder variant="today" />);

    expect(await screen.findByTestId('daily-child-switcher')).toHaveTextContent('老二 ▾');
    expect(screen.queryByTestId('child-picker')).toBeNull();
    expect(await screen.findByTestId('daily-today-empty')).toHaveTextContent(/记录给 老二/);

    await fireEvent.press(screen.getByTestId('daily-child-switcher'));
    expect(screen.getByTestId('child-option-2').props.accessibilityLabel).toBe('老二');
    expect(screen.getByTestId('child-option-2').props.accessibilityState).toEqual({
      selected: true,
    });
  });

  it('reopens the picker when the persisted child disappears and other children remain', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    useApi([second, third]);

    await renderScreen(<DailyPlaceholder variant="today" />);

    expect(await screen.findByTestId('child-picker')).toBeTruthy();
    expect(screen.queryByTestId('child-option-1')).toBeNull();
    await waitFor(() =>
      expect(AsyncStorage.removeItem).toHaveBeenCalledWith(CURRENT_CHILD_STORAGE_KEY),
    );

    await fireEvent.press(screen.getByTestId('child-option-3'));
    await waitFor(() => expect(screen.queryByTestId('child-picker')).toBeNull());
    expect(screen.getByTestId('daily-child-switcher')).toHaveTextContent('老三 ▾');
  });

  it('falls back to the remaining child after the persisted child is deleted', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, String(doudou.member_id));
    useApi([second]);

    await renderScreen(<DailyPlaceholder variant="stats" />);

    expect(await screen.findByTestId('daily-current-child')).toHaveTextContent('老二');
    expect(await screen.findByTestId('daily-stats-empty')).toHaveTextContent(/老二/);
    await waitFor(() =>
      expect(AsyncStorage.setItem).toHaveBeenCalledWith(CURRENT_CHILD_STORAGE_KEY, '2'),
    );
  });

  it('offers a manage-members entry that leaves the picker', async () => {
    useApi([doudou, second]);

    await renderScreen(<DailyPlaceholder variant="today" />);
    expect(await screen.findByTestId('child-picker')).toBeTruthy();

    await fireEvent.press(screen.getByTestId('child-picker-manage-members'));

    expect(mockRouter.push).toHaveBeenCalledWith('/members');
    expect(screen.queryByTestId('child-picker')).toBeNull();
  });
});
