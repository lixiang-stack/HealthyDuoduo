import { CARE_EVENT_FIXTURES } from './careEventFixtures';
import { InMemoryCareEventRepository } from './careEventRepository';
import {
  careEventOverlapsRange,
  compareCareEventsDesc,
  type CareEvent,
  type CareEventRange,
} from './careEvents';

const TODAY: CareEventRange = {
  from: '2026-11-23T00:00:00+08:00',
  to: '2026-11-24T00:00:00+08:00',
};

function fixtureById(id: number): CareEvent {
  const event = CARE_EVENT_FIXTURES.find((candidate) => candidate.id === id);
  if (!event) {
    throw new Error(`missing fixture ${id}`);
  }
  return event;
}

describe('care event fixtures', () => {
  it('cover every type and the boundary cases the UI tickets need', () => {
    expect(new Set(CARE_EVENT_FIXTURES.map((event) => event.type))).toEqual(
      new Set(['feed', 'diaper', 'sleep', 'solid']),
    );
    expect(CARE_EVENT_FIXTURES.some((event) => event.note === null)).toBe(true);
    expect(
      CARE_EVENT_FIXTURES.some(
        (event) =>
          (event.type === 'feed' && event.amount_ml === null) ||
          (event.type === 'solid' && event.amount_g === null),
      ),
    ).toBe(true);
    expect(
      CARE_EVENT_FIXTURES.some(
        (event) => event.type === 'sleep' && event.ended_at?.startsWith('2026-11-23') === true,
      ),
    ).toBe(true);
  });

  it('never carry caregiver or device attribution fields', () => {
    for (const event of CARE_EVENT_FIXTURES) {
      expect(event).not.toHaveProperty('recorded_by');
      expect(event).not.toHaveProperty('device_id');
      expect(event).not.toHaveProperty('caregiver');
    }
  });
});

describe('care event range', () => {
  it('includes a cross-midnight event when it overlaps the requested day', () => {
    expect(careEventOverlapsRange(fixtureById(4), TODAY)).toBe(true);
  });

  it('includes an ongoing event in any later range', () => {
    const ongoing = { started_at: '2026-11-22T23:40:00+08:00', ended_at: null };

    expect(careEventOverlapsRange(ongoing, TODAY)).toBe(true);
  });

  it('excludes an event that ended exactly at the inclusive from edge', () => {
    const endedAtFrom = {
      started_at: '2026-11-22T23:00:00+08:00',
      ended_at: TODAY.from,
    };

    expect(careEventOverlapsRange(endedAtFrom, TODAY)).toBe(false);
  });

  it('includes a zero-duration point event exactly at from', () => {
    const pointAtFrom = { started_at: TODAY.from, ended_at: TODAY.from };

    expect(careEventOverlapsRange(pointAtFrom, TODAY)).toBe(true);
  });

  it('treats from as inclusive and to as exclusive', () => {
    const feed = fixtureById(1);
    expect(
      careEventOverlapsRange(feed, {
        from: feed.started_at,
        to: '2026-11-23T14:06:00+08:00',
      }),
    ).toBe(true);
    expect(
      careEventOverlapsRange(feed, {
        from: '2026-11-23T14:00:00+08:00',
        to: feed.started_at,
      }),
    ).toBe(false);
  });

  it('orders events newest first, with id as a stable tie-breaker', () => {
    const first = fixtureById(1);
    const second = { ...fixtureById(2), started_at: first.started_at, id: 99 };
    expect([second, first].sort(compareCareEventsDesc).map((event) => event.id)).toEqual([99, 1]);
  });
});

describe('InMemoryCareEventRepository', () => {
  it('lists one member events in reverse chronological order', async () => {
    const repository = new InMemoryCareEventRepository(CARE_EVENT_FIXTURES);

    const events = await repository.list({ member_id: 1, ...TODAY });

    expect(events.map((event) => event.id)).toEqual([1, 5, 2, 3, 4]);
  });

  it('filters by member and by range', async () => {
    const repository = new InMemoryCareEventRepository([
      ...CARE_EVENT_FIXTURES,
      {
        ...fixtureById(1),
        id: 7,
        member_id: 2,
        started_at: '2026-11-23T15:00:00+08:00',
      },
    ]);

    const memberOne = await repository.list({ member_id: 1, ...TODAY });
    const previousDay = await repository.list({
      member_id: 1,
      from: '2026-11-22T00:00:00+08:00',
      to: '2026-11-23T00:00:00+08:00',
    });

    expect(memberOne.map((event) => event.id)).not.toContain(7);
    expect(previousDay.map((event) => event.id)).toEqual([4]);
  });

  it('creates events as parent records with a fresh id', async () => {
    const repository = new InMemoryCareEventRepository(CARE_EVENT_FIXTURES);

    const created = await repository.create({
      member_id: 1,
      type: 'solid',
      started_at: '2026-11-23T17:30:00+08:00',
      ended_at: null,
      note: null,
      food: '南瓜泥',
      amount_g: null,
    });

    expect(created).toMatchObject({ id: 6, member_id: 1, source: 'parent', type: 'solid' });
    expect((await repository.list({ member_id: 1, ...TODAY }))[0]).toEqual(created);
  });

  it('updates editable content while preserving identity and member', async () => {
    const repository = new InMemoryCareEventRepository(CARE_EVENT_FIXTURES);

    const updated = await repository.update(2, {
      type: 'feed',
      started_at: '2026-11-23T09:12:00+08:00',
      ended_at: '2026-11-23T09:32:00+08:00',
      note: '换边',
      method: 'breast',
      amount_ml: null,
      duration_min: 20,
    });

    expect(updated).toMatchObject({
      id: 2,
      member_id: 1,
      source: 'parent',
      note: '换边',
      duration_min: 20,
    });
  });

  it('removes an event and rejects unknown ids', async () => {
    const repository = new InMemoryCareEventRepository(CARE_EVENT_FIXTURES);

    await repository.remove(3);

    expect((await repository.list({ member_id: 1, ...TODAY })).map((event) => event.id)).not.toContain(
      3,
    );
    await expect(repository.remove(999)).rejects.toThrow('care event 999 not found');
  });
});
