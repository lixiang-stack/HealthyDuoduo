import { fireEvent, render, screen, waitFor } from '@testing-library/react-native';
import type { ComponentProps } from 'react';

import { makeFeedEvent, makeMember } from '../test/fixtures';
import type { CareEvent } from '../care/careEvents';
import { InMemoryCareEventRepository } from '../care/careEventRepository';
import { CareEventProvider } from '../care/CareEventProvider';
import { RecordSheet } from './RecordSheet';

const doudou = makeMember({ member_id: 1, name: '豆豆', is_child: true });
const second = makeMember({ member_id: 2, name: '老二', is_child: true });
const mom = makeMember({ member_id: 9, name: '妈妈', is_child: false });

const WIDE_RANGE = {
  from: '2000-01-01T00:00:00.000Z',
  to: '2100-01-01T00:00:00.000Z',
};

async function renderSheet(
  props: Partial<ComponentProps<typeof RecordSheet>> = {},
  repository = new InMemoryCareEventRepository(),
) {
  const view = await render(
    <RecordSheet
      type="feed"
      members={[doudou, second, mom]}
      initialTargetMemberId={doudou.member_id}
      onClose={jest.fn()}
      onSaved={jest.fn()}
      {...props}
    />,
    {
      wrapper: ({ children }) => (
        <CareEventProvider repository={repository}>{children}</CareEventProvider>
      ),
    },
  );
  return { ...view, repository };
}

describe('RecordSheet target', () => {
  it('shows the record title, frozen target and the parent-record source', async () => {
    await renderSheet();

    expect(screen.getByTestId('record-title')).toHaveTextContent('记录喂奶');
    expect(screen.getByTestId('record-target')).toHaveTextContent('记录给 豆豆');
    expect(screen.getByTestId('record-current-child-tag')).toHaveTextContent('当前孩子');
  });

  it('does not show child chips when only one child exists', async () => {
    await renderSheet({ members: [doudou, mom] });

    expect(screen.queryByTestId('record-target-option-1')).toBeNull();
    expect(screen.getByTestId('record-target')).toHaveTextContent('记录给 豆豆');
  });

  it('lists only children as chips and lets the user change the target', async () => {
    await renderSheet();

    expect(screen.getByTestId('record-target-option-1')).toHaveTextContent('豆豆');
    expect(screen.getByTestId('record-target-option-2')).toHaveTextContent('老二');
    expect(screen.queryByTestId('record-target-option-9')).toBeNull();

    await fireEvent.press(screen.getByTestId('record-target-option-2'));

    expect(screen.getByTestId('record-target')).toHaveTextContent('记录给 老二');
    expect(screen.getByTestId('record-target-option-2').props.accessibilityState).toEqual({
      selected: true,
    });
    expect(screen.getByTestId('record-current-child-tag')).toHaveTextContent('当前孩子');
  });

  it('disables saving when several children exist and no target is frozen', async () => {
    await renderSheet({ initialTargetMemberId: undefined });

    expect(screen.getByTestId('record-target')).toHaveTextContent('记录给 未选择');
    expect(screen.getByTestId('record-submit').props.accessibilityState).toEqual({
      disabled: true,
    });
  });

  it('keeps the target frozen when the parent later passes a different current child', async () => {
    const repository = new InMemoryCareEventRepository();
    const onSaved = jest.fn();
    const view = await renderSheet({ type: 'diaper', onSaved }, repository);

    await view.rerender(
      <RecordSheet
        type="diaper"
        members={[doudou, second, mom]}
        initialTargetMemberId={second.member_id}
        onClose={jest.fn()}
        onSaved={onSaved}
      />,
    );

    expect(screen.getByTestId('record-target')).toHaveTextContent('记录给 豆豆');
    await fireEvent.press(screen.getByTestId('record-submit'));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    const events = await repository.list({
      member_id: doudou.member_id,
      from: '2000-01-01T00:00:00.000Z',
      to: '2100-01-01T00:00:00.000Z',
    });
    expect(events).toHaveLength(1);
    expect(events[0].member_id).toBe(doudou.member_id);
  });
});

describe('RecordSheet fields and saving', () => {
  it('labels the event as a parent record and starts the time at now', async () => {
    await renderSheet();

    expect(screen.getByTestId('record-source-tag')).toHaveTextContent('家长记录');
    expect(screen.getByTestId('record-time').props.value).toMatch(/^\d{2}:\d{2}$/);
  });

  it('requires a feeding method and an HH:MM time', async () => {
    const repository = new InMemoryCareEventRepository();
    const onSaved = jest.fn();
    await renderSheet({ onSaved }, repository);

    await fireEvent.press(screen.getByTestId('record-submit'));
    expect(screen.getByTestId('record-method-error')).toHaveTextContent('请选择喂养方式');

    await fireEvent.press(screen.getByTestId('record-method-formula'));
    await fireEvent.changeText(screen.getByTestId('record-time'), '25:00');
    await fireEvent.press(screen.getByTestId('record-submit'));

    expect(screen.getByTestId('record-time-error')).toHaveTextContent(
      '请填写 HH:MM 格式的时间',
    );
    expect(onSaved).not.toHaveBeenCalled();
    expect(await repository.list({ member_id: 1, ...WIDE_RANGE })).toHaveLength(0);
  });

  it('creates a formula feed with amount, note and the frozen member', async () => {
    const repository = new InMemoryCareEventRepository();
    const onSaved = jest.fn();
    await renderSheet({ onSaved }, repository);

    await fireEvent.press(screen.getByTestId('record-method-formula'));
    await fireEvent.changeText(screen.getByTestId('record-amount'), '0');
    await fireEvent.press(screen.getByTestId('record-submit'));
    expect(screen.getByTestId('record-amount-error')).toHaveTextContent('奶量需为正数');

    await fireEvent.changeText(screen.getByTestId('record-amount'), '90');
    await fireEvent.changeText(screen.getByTestId('record-note'), '  剩余 20 ml ');
    await fireEvent.press(screen.getByTestId('record-submit'));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    const [created] = await repository.list({ member_id: 1, ...WIDE_RANGE });
    expect(created).toMatchObject({
      member_id: 1,
      type: 'feed',
      method: 'formula',
      amount_ml: 90,
      duration_min: null,
      note: '剩余 20 ml',
      source: 'parent',
    });
  });

  it('creates a diaper event with the selected type', async () => {
    const repository = new InMemoryCareEventRepository();
    const onSaved = jest.fn();
    await renderSheet({ members: [doudou, mom], type: 'diaper', onSaved }, repository);

    expect(screen.getByTestId('record-title')).toHaveTextContent('记录尿布');
    await fireEvent.press(screen.getByTestId('record-diaper-mixed'));
    await fireEvent.press(screen.getByTestId('record-submit'));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    const [created] = await repository.list({ member_id: 1, ...WIDE_RANGE });
    expect(created).toMatchObject({ type: 'diaper', diaper_type: 'mixed' });
  });

  it('creates a sleep event with duration and derives its end', async () => {
    const repository = new InMemoryCareEventRepository();
    const onSaved = jest.fn();
    await renderSheet({ members: [doudou, mom], type: 'sleep', onSaved }, repository);

    expect(screen.getByTestId('record-title')).toHaveTextContent('记录睡觉');
    await fireEvent.changeText(screen.getByTestId('record-duration'), '45');
    await fireEvent.press(screen.getByTestId('record-submit'));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    const [created] = await repository.list({ member_id: 1, ...WIDE_RANGE });
    expect(created).toMatchObject({ type: 'sleep', duration_min: 45 });
    expect(created.type === 'sleep' && created.ended_at).toBeTruthy();
  });

  it('requires solid food and creates the solid event with its amount', async () => {
    const repository = new InMemoryCareEventRepository();
    const onSaved = jest.fn();
    await renderSheet({ members: [doudou, mom], type: 'solid', onSaved }, repository);

    expect(screen.getByTestId('record-title')).toHaveTextContent('记录辅食');
    await fireEvent.press(screen.getByTestId('record-submit'));
    expect(screen.getByTestId('record-food-error')).toHaveTextContent('请填写食物名称');
    await fireEvent.changeText(screen.getByTestId('record-food'), '米粉');
    await fireEvent.changeText(screen.getByTestId('record-amount-g'), '40');
    await fireEvent.press(screen.getByTestId('record-submit'));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    const [created] = await repository.list({ member_id: 1, ...WIDE_RANGE });
    expect(created).toMatchObject({ type: 'solid', food: '米粉', amount_g: 40 });
  });

  it('edits an existing event without changing its owner and can delete it', async () => {
    const feed = makeFeedEvent({ note: '旧备注' });
    const repository = new InMemoryCareEventRepository([feed]);
    const onSaved = jest.fn();
    await renderSheet({ editingEvent: feed, onSaved }, repository);

    expect(screen.getByTestId('record-title')).toHaveTextContent('修改记录');
    expect(screen.getByTestId('record-target')).toHaveTextContent('记录给 豆豆');
    expect(screen.queryByTestId('record-target-option-2')).toBeNull();
    expect(screen.getByTestId('record-method-formula').props.accessibilityState).toEqual({
      selected: true,
    });
    expect(screen.getByTestId('record-amount').props.value).toBe('120');

    await fireEvent.changeText(screen.getByTestId('record-note'), '换边');
    await fireEvent.press(screen.getByTestId('record-submit'));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    const [updated] = await repository.list({ member_id: 1, ...WIDE_RANGE });
    expect(updated).toMatchObject({
      id: feed.id,
      member_id: feed.member_id,
      source: 'parent',
      note: '换边',
    });

    await fireEvent.press(screen.getByTestId('record-delete'));
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(2));
    expect(await repository.list({ member_id: 1, ...WIDE_RANGE })).toHaveLength(0);
  });
});

describe('RecordSheet duplicate feed guard', () => {
  function previousFeed(minutesAgo: number, overrides: Partial<Extract<CareEvent, { type: 'feed' }>> = {}) {
    return makeFeedEvent({
      id: 10,
      started_at: new Date(Date.now() - minutesAgo * 60000).toISOString(),
      ...overrides,
    });
  }

  it('asks before adding a second feed inside the five-minute window', async () => {
    const repository = new InMemoryCareEventRepository([previousFeed(3)]);
    const onSaved = jest.fn();
    await renderSheet({ onSaved }, repository);

    await fireEvent.press(screen.getByTestId('record-method-formula'));
    await fireEvent.press(screen.getByTestId('record-submit'));

    expect(await screen.findByTestId('duplicate-feed-confirm')).toBeTruthy();
    expect(screen.getByTestId('duplicate-feed-message')).toHaveTextContent(
      /刚刚已有一条喂养记录（\d{2}:\d{2} · 奶粉 120 ml），仍然添加？/,
    );
    expect(onSaved).not.toHaveBeenCalled();
  });

  it('adds the second feed when the parent chooses 仍然添加', async () => {
    const repository = new InMemoryCareEventRepository([previousFeed(3)]);
    const onSaved = jest.fn();
    await renderSheet({ onSaved }, repository);

    await fireEvent.press(screen.getByTestId('record-method-formula'));
    await fireEvent.press(screen.getByTestId('record-submit'));
    await screen.findByTestId('duplicate-feed-confirm');
    await fireEvent.press(screen.getByTestId('duplicate-feed-continue'));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(await repository.list({ member_id: 1, ...WIDE_RANGE })).toHaveLength(2);
  });

  it('keeps the form content and writes nothing when the parent chooses 返回修改', async () => {
    const repository = new InMemoryCareEventRepository([previousFeed(3)]);
    const onSaved = jest.fn();
    await renderSheet({ onSaved }, repository);

    await fireEvent.press(screen.getByTestId('record-method-formula'));
    await fireEvent.changeText(screen.getByTestId('record-note'), '保留这条备注');
    await fireEvent.press(screen.getByTestId('record-submit'));
    await screen.findByTestId('duplicate-feed-confirm');
    await fireEvent.press(screen.getByTestId('duplicate-feed-back'));

    expect(screen.queryByTestId('duplicate-feed-confirm')).toBeNull();
    expect(screen.getByTestId('record-note').props.value).toBe('保留这条备注');
    expect(onSaved).not.toHaveBeenCalled();
    expect(await repository.list({ member_id: 1, ...WIDE_RANGE })).toHaveLength(1);
  });

  it('does not ask outside the window', async () => {
    const repository = new InMemoryCareEventRepository([previousFeed(6)]);
    const onSaved = jest.fn();
    await renderSheet({ onSaved }, repository);

    await fireEvent.press(screen.getByTestId('record-method-formula'));
    await fireEvent.press(screen.getByTestId('record-submit'));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(screen.queryByTestId('duplicate-feed-confirm')).toBeNull();
  });

  it('does not ask when the previous feed belongs to another child', async () => {
    const repository = new InMemoryCareEventRepository([previousFeed(3)]);
    const onSaved = jest.fn();
    await renderSheet({ initialTargetMemberId: second.member_id, onSaved }, repository);

    await fireEvent.press(screen.getByTestId('record-method-formula'));
    await fireEvent.press(screen.getByTestId('record-submit'));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(screen.queryByTestId('duplicate-feed-confirm')).toBeNull();
    expect(await repository.list({ member_id: 2, ...WIDE_RANGE })).toHaveLength(1);
  });

  it('does not ask when editing the feed itself', async () => {
    const existing = previousFeed(3, { id: 1 });
    const repository = new InMemoryCareEventRepository([existing]);
    const onSaved = jest.fn();
    await renderSheet({ editingEvent: existing, onSaved }, repository);

    await fireEvent.press(screen.getByTestId('record-submit'));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(screen.queryByTestId('duplicate-feed-confirm')).toBeNull();
    expect(await repository.list({ member_id: 1, ...WIDE_RANGE })).toHaveLength(1);
  });

  it('asks when an edited feed moves into another feed’s window and updates on 仍然添加', async () => {
    const editing = previousFeed(0, { id: 1 });
    const other = previousFeed(3, { id: 10 });
    const repository = new InMemoryCareEventRepository([editing, other]);
    const onSaved = jest.fn();
    await renderSheet({ editingEvent: editing, onSaved }, repository);

    await fireEvent.press(screen.getByTestId('record-submit'));

    expect(await screen.findByTestId('duplicate-feed-confirm')).toBeTruthy();
    await fireEvent.press(screen.getByTestId('duplicate-feed-continue'));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    const events = await repository.list({ member_id: 1, ...WIDE_RANGE });
    expect(events.map((event) => event.id).sort()).toEqual([1, 10]);
  });
});
