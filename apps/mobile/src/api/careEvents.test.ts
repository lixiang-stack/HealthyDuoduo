import type { FeedCareEventBody, SleepCareEventBody } from '../care/careEvents';
import type { ApiClient } from './client';
import {
  createCareEvent,
  deleteCareEvent,
  listCareEvents,
  updateCareEvent,
} from './careEvents';

const RANGE = {
  from: '2026-11-18T00:00:00.000Z',
  to: '2026-11-25T00:00:00.000Z',
};

const feedWire = {
  id: 1,
  member_id: 2,
  type: 'feed',
  source: 'parent',
  started_at: '2026-11-24T14:05:00+08:00',
  ended_at: null,
  note: null,
  method: 'formula',
};

const sleepWire = {
  id: 2,
  member_id: 2,
  type: 'sleep',
  source: 'parent',
  started_at: '2026-11-24T12:00:00+08:00',
  ended_at: '2026-11-24T13:30:00+08:00',
  note: '午睡',
  duration_min: 90,
};

const solidWire = {
  id: 3,
  member_id: 2,
  type: 'solid',
  source: 'parent',
  started_at: '2026-11-24T11:00:00+08:00',
  ended_at: null,
  note: null,
  food: '米粉',
};

function okClient(method: 'GET' | 'POST' | 'PATCH' | 'DELETE', data: unknown): ApiClient {
  const fn = jest.fn(async () => ({
    data,
    error: undefined,
    response: { ok: true, status: 200 } as Response,
  }));
  return { [method]: fn } as unknown as ApiClient;
}

describe('care-event wire mapping', () => {
  it('lists the requested range and fills omitted optional fields with null', async () => {
    const client = okClient('GET', { events: [feedWire, sleepWire, solidWire] });

    const events = await listCareEvents(client, 2, RANGE);

    expect(client.GET).toHaveBeenCalledWith('/members/{member_id}/care-events', {
      params: { path: { member_id: 2 }, query: RANGE },
    });
    expect(events).toEqual([
      {
        ...feedWire,
        amount_ml: null,
        duration_min: null,
      },
      sleepWire,
      {
        ...solidWire,
        amount_g: null,
      },
    ]);
  });

  it('sends only the editable body when creating an event', async () => {
    const client = okClient('POST', feedWire);
    const body: FeedCareEventBody = {
      type: 'feed',
      started_at: '2026-11-24T14:05:00+08:00',
      ended_at: null,
      note: '剩下 20 ml',
      method: 'formula',
      amount_ml: 90,
      duration_min: null,
    };

    const event = await createCareEvent(client, 2, { member_id: 2, ...body });

    expect(client.POST).toHaveBeenCalledWith('/members/{member_id}/care-events', {
      params: { path: { member_id: 2 } },
      body,
    });
    expect(event).toEqual({ ...feedWire, amount_ml: null, duration_min: null });
  });

  it('replaces the editable body when updating an event', async () => {
    const client = okClient('PATCH', sleepWire);
    const body: SleepCareEventBody = {
      type: 'sleep',
      started_at: '2026-11-24T12:00:00+08:00',
      ended_at: '2026-11-24T13:30:00+08:00',
      note: '午睡',
      duration_min: 90,
    };

    const event = await updateCareEvent(client, 2, body);

    expect(client.PATCH).toHaveBeenCalledWith('/care-events/{event_id}', {
      params: { path: { event_id: 2 } },
      body,
    });
    expect(event).toEqual(sleepWire);
  });

  it('resolves delete without a response body', async () => {
    const client = okClient('DELETE', undefined);

    await expect(deleteCareEvent(client, 7)).resolves.toBeUndefined();

    expect(client.DELETE).toHaveBeenCalledWith('/care-events/{event_id}', {
      params: { path: { event_id: 7 } },
    });
  });

  it('surfaces the contract error message', async () => {
    const client = {
      GET: async () => ({
        data: undefined,
        error: { message: '成员不存在' },
        response: { ok: false, status: 404 } as Response,
      }),
    } as unknown as ApiClient;

    await expect(listCareEvents(client, 99, RANGE)).rejects.toThrow('成员不存在');
  });
});
