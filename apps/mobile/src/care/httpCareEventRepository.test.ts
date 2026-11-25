import type { ApiClient } from '../api/client';
import { TEST_BASE_URL } from '../test/fakeApi';
import type { CareEventCreate, CareEventQuery } from './careEvents';
import {
  CARE_EVENTS_UNCONFIGURED_MESSAGE,
  HttpCareEventRepository,
} from './httpCareEventRepository';

const QUERY: CareEventQuery = {
  member_id: 2,
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

const createBody: CareEventCreate = {
  member_id: 2,
  type: 'feed',
  started_at: '2026-11-24T14:05:00+08:00',
  ended_at: null,
  note: null,
  method: 'formula',
  amount_ml: 120,
  duration_min: null,
};

function clientReturning(data: unknown): ApiClient {
  return {
    GET: jest.fn(async () => ({
      data,
      error: undefined,
      response: { ok: true, status: 200 } as Response,
    })),
  } as unknown as ApiClient;
}

describe('HttpCareEventRepository', () => {
  it('throws a recognizable error when no backend is configured', async () => {
    const repository = new HttpCareEventRepository(async () => undefined);

    await expect(repository.list(QUERY)).rejects.toThrow(
      CARE_EVENTS_UNCONFIGURED_MESSAGE,
    );
    await expect(repository.create(createBody)).rejects.toThrow(
      CARE_EVENTS_UNCONFIGURED_MESSAGE,
    );
    await expect(repository.update(1, createBody)).rejects.toThrow(
      CARE_EVENTS_UNCONFIGURED_MESSAGE,
    );
    await expect(repository.remove(1)).rejects.toThrow(
      CARE_EVENTS_UNCONFIGURED_MESSAGE,
    );
  });

  it('re-resolves the client on every call', async () => {
    const first = clientReturning({ events: [feedWire] });
    const second = clientReturning({ events: [] });
    const resolveClient = jest
      .fn()
      .mockResolvedValueOnce({ client: first, baseUrl: TEST_BASE_URL })
      .mockResolvedValueOnce({ client: second, baseUrl: TEST_BASE_URL });
    const repository = new HttpCareEventRepository(resolveClient);

    await expect(repository.list(QUERY)).resolves.toHaveLength(1);
    await expect(repository.list(QUERY)).resolves.toEqual([]);

    expect(resolveClient).toHaveBeenCalledTimes(2);
    expect(first.GET).toHaveBeenCalledTimes(1);
    expect(second.GET).toHaveBeenCalledTimes(1);
  });

  it('maps list results from the wire shape', async () => {
    const client = clientReturning({ events: [feedWire] });
    const repository = new HttpCareEventRepository(async () => ({
      client,
      baseUrl: TEST_BASE_URL,
    }));

    await expect(repository.list(QUERY)).resolves.toEqual([
      { ...feedWire, amount_ml: null, duration_min: null },
    ]);
  });

  it('sends only from / to as query parameters', async () => {
    const get = jest.fn(async () => ({
      data: { events: [] },
      error: undefined,
      response: { ok: true, status: 200 } as Response,
    }));
    const client = { GET: get } as unknown as ApiClient;
    const repository = new HttpCareEventRepository(async () => ({
      client,
      baseUrl: TEST_BASE_URL,
    }));

    await repository.list(QUERY);

    expect(get).toHaveBeenCalledWith('/members/{member_id}/care-events', {
      params: {
        path: { member_id: QUERY.member_id },
        query: { from: QUERY.from, to: QUERY.to },
      },
    });
  });

  it('surfaces server errors as plain Error messages', async () => {
    const client = {
      GET: async () => ({
        data: undefined,
        error: { message: '后端不可达' },
        response: { ok: false, status: 500 } as Response,
      }),
    } as unknown as ApiClient;
    const repository = new HttpCareEventRepository(async () => ({
      client,
      baseUrl: TEST_BASE_URL,
    }));

    await expect(repository.list(QUERY)).rejects.toThrow('后端不可达');
  });
});
