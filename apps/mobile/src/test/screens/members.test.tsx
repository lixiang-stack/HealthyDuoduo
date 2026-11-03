import AsyncStorage from '@react-native-async-storage/async-storage';
import { fireEvent, screen, waitFor } from '@testing-library/react-native';

import { resolveApiClient } from '../../api/resolveClient';
import MembersScreen from '../../app/members';
import { CURRENT_MEMBER_STORAGE_KEY } from '../../members/currentMemberStore';
import { createFakeApiClient, TEST_BASE_URL, type FakeApiOptions } from '../fakeApi';
import { makeGrowthRecord, makeMember, makeReportSummary } from '../fixtures';
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

  it('requires name, sex and a confirmed birth date, then submits the wheel date', async () => {
    const createHandler = jest.fn((_options?: FakeApiOptions) =>
      makeMember({ member_id: 2, name: '小虎' }),
    );
    useApi({
      GET: {
        '/members': () => ({ members: [] }),
      },
      POST: { '/members': createHandler },
    });

    await renderScreen(<MembersScreen />);
    expect(await screen.findByText(/还没有家庭成员/)).toBeTruthy();

    await fireEvent.press(screen.getByTestId('members-create'));
    expect(createHandler).not.toHaveBeenCalled();
    expect(screen.getByTestId('members-name-error')).toHaveTextContent('请填写称呼');
    expect(screen.getByTestId('members-sex-error')).toHaveTextContent('请选择性别');
    expect(screen.getByTestId('members-birth-date-error')).toHaveTextContent('请选择出生日期');

    await fireEvent.changeText(screen.getByTestId('members-name-input'), '小虎');
    await fireEvent.press(screen.getByTestId('members-sex-field'));
    await fireEvent.press(screen.getByTestId('members-sex-option-female'));
    await fireEvent.press(screen.getByTestId('members-create'));
    expect(createHandler).not.toHaveBeenCalled();
    expect(screen.getByTestId('members-birth-date-error')).toHaveTextContent('请选择出生日期');

    await fireEvent.press(screen.getByTestId('members-birth-date-field'));
    await fireEvent.press(screen.getByTestId('members-birth-year-wheel-option-2019'));
    await fireEvent.press(screen.getByTestId('members-birth-month-wheel-option-5'));
    await fireEvent.press(screen.getByTestId('members-birth-day-wheel-option-20'));
    await fireEvent.press(screen.getByTestId('members-birth-confirm'));
    await fireEvent.press(screen.getByTestId('members-create'));

    await waitFor(() => expect(createHandler).toHaveBeenCalledTimes(1));
    expect(createHandler.mock.calls[0][0]).toMatchObject({
      body: {
        name: '小虎',
        sex: 'female',
        birth_date: '2019-05-20',
      },
    });
  });

  it('edits sex and birth date through a partial PATCH', async () => {
    let members = [
      makeMember({
        member_id: 1,
        name: '小明',
        sex: 'female',
        birth_date: '2020-01-01',
      }),
    ];
    const patchHandler = jest.fn((options?: FakeApiOptions) => {
      const body = options?.body as Record<string, unknown>;
      members = [
        {
          ...members[0],
          name: String(body.name),
          sex: body.sex as 'male' | 'female',
          birth_date: String(body.birth_date),
        },
      ];
      return members[0];
    });
    useApi({
      GET: { '/members': () => ({ members }) },
      PATCH: { '/members/{member_id}': patchHandler },
    });

    await renderScreen(<MembersScreen />);
    expect(await screen.findByText('小明')).toBeTruthy();

    await fireEvent.press(screen.getByText('编辑'));
    await fireEvent.press(screen.getByTestId('member-edit-1-sex-field'));
    await fireEvent.press(screen.getByTestId('member-edit-1-sex-option-male'));
    await fireEvent.press(screen.getByTestId('member-edit-1-birth-date-field'));
    await fireEvent.press(screen.getByTestId('member-edit-1-birth-confirm'));
    await fireEvent.press(screen.getByTestId('member-edit-save-1'));

    await waitFor(() => expect(patchHandler).toHaveBeenCalledTimes(1));
    expect(patchHandler.mock.calls[0][0]).toMatchObject({
      params: { path: { member_id: 1 } },
      body: {
        name: '小明',
        sex: 'male',
        birth_date: '2020-01-01',
      },
    });
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

  it('records a growth measurement and shows the latest record in the member row', async () => {
    let members = [makeMember({ member_id: 1, name: '小明', latest_growth: null })];
    const createGrowthHandler = jest.fn((options?: FakeApiOptions) => {
      const body = options?.body as {
        measured_on: string;
        height_cm?: number;
        weight_kg?: number;
        source: 'measured' | 'report' | 'manual';
      };
      const bmi =
        body.height_cm && body.weight_kg
          ? Math.round((body.weight_kg / (body.height_cm / 100) ** 2) * 10) / 10
          : null;
      const record = makeGrowthRecord({
        member_id: 1,
        measured_on: body.measured_on,
        height_cm: body.height_cm ?? null,
        weight_kg: body.weight_kg ?? null,
        source: body.source,
        bmi,
      });
      members = [{ ...members[0], latest_growth: record }];
      return record;
    });
    useApi({
      GET: { '/members': () => ({ members }) },
      POST: { '/members/{member_id}/growth': createGrowthHandler },
    });

    await renderScreen(<MembersScreen />);
    expect(await screen.findByText('小明')).toBeTruthy();

    fireEvent.press(screen.getByTestId('member-growth-action-1'));
    await screen.findByTestId('growth-measured-on');
    await fireEvent.changeText(screen.getByTestId('growth-measured-on'), '');
    await fireEvent.press(screen.getByTestId('growth-submit'));
    expect(screen.getByTestId('growth-measured-on-error')).toHaveTextContent(
      '请填写有效的测量日期（不晚于今天）',
    );
    expect(screen.getByTestId('growth-measurement-error')).toHaveTextContent(
      '身高和体重至少填一项',
    );
    expect(createGrowthHandler).not.toHaveBeenCalled();

    await fireEvent.changeText(screen.getByTestId('growth-measured-on'), '2026-10-01');
    await fireEvent.changeText(screen.getByTestId('growth-height'), '118.5');
    await fireEvent.changeText(screen.getByTestId('growth-weight'), '22.4');
    await fireEvent.press(screen.getByTestId('growth-source-measured'));
    await fireEvent.press(screen.getByTestId('growth-submit'));

    await waitFor(() => expect(createGrowthHandler).toHaveBeenCalledTimes(1));
    expect(createGrowthHandler.mock.calls[0][0]).toMatchObject({
      params: { path: { member_id: 1 } },
      body: {
        measured_on: '2026-10-01',
        height_cm: 118.5,
        weight_kg: 22.4,
        source: 'measured',
      },
    });
    expect(await screen.findByTestId('member-growth-1')).toHaveTextContent(
      '实测 · 身高 118.5 cm · 体重 22.4 kg · BMI 16.0 · 2026-10-01',
    );
  });
});
