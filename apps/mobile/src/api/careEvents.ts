// Care-event HTTP adapter (spec §7.1). Generated schema types are the wire boundary;
// the hand-written CareEvent union in ./careEvents stays the UI / domain seam so the
// timeline, stats and record sheet never depend on the OpenAPI oneOf details.
import type {
  CareEvent,
  CareEventBody,
  CareEventCreate,
  CareEventRange,
  CareEventUpdate,
} from '../care/careEvents';
import type { ApiClient } from './client';
import { assertOk, unwrap } from './client';
import type { components } from './schema';

export type CareEventWire = components['schemas']['CareEvent'];
export type CareEventCreateWire = components['schemas']['CareEventCreate'];

export async function listCareEvents(
  client: ApiClient,
  memberId: number,
  range: CareEventRange,
): Promise<CareEvent[]> {
  return unwrap(
    await client.GET('/members/{member_id}/care-events', {
      params: { path: { member_id: memberId }, query: { ...range } },
    }),
  ).events.map(toCareEvent);
}

export async function createCareEvent(
  client: ApiClient,
  memberId: number,
  body: CareEventCreate,
): Promise<CareEvent> {
  return toCareEvent(
    unwrap(
      await client.POST('/members/{member_id}/care-events', {
        params: { path: { member_id: memberId } },
        body: toWireBody(body),
      }),
    ),
  );
}

export async function updateCareEvent(
  client: ApiClient,
  eventId: number,
  body: CareEventUpdate,
): Promise<CareEvent> {
  return toCareEvent(
    unwrap(
      await client.PATCH('/care-events/{event_id}', {
        params: { path: { event_id: eventId } },
        body: toWireBody(body),
      }),
    ),
  );
}

export async function deleteCareEvent(client: ApiClient, eventId: number): Promise<void> {
  assertOk(
    await client.DELETE('/care-events/{event_id}', {
      params: { path: { event_id: eventId } },
    }),
  );
}

function toWireBody(body: CareEventBody): CareEventCreateWire {
  switch (body.type) {
    case 'feed':
      return {
        type: 'feed',
        started_at: body.started_at,
        ended_at: body.ended_at,
        note: body.note,
        method: body.method,
        amount_ml: body.amount_ml,
        duration_min: body.duration_min,
      };
    case 'diaper':
      return {
        type: 'diaper',
        started_at: body.started_at,
        ended_at: body.ended_at,
        note: body.note,
        diaper_type: body.diaper_type,
      };
    case 'sleep':
      return {
        type: 'sleep',
        started_at: body.started_at,
        ended_at: body.ended_at,
        note: body.note,
        duration_min: body.duration_min,
      };
    case 'solid':
      return {
        type: 'solid',
        started_at: body.started_at,
        ended_at: body.ended_at,
        note: body.note,
        food: body.food,
        amount_g: body.amount_g,
      };
  }
}

/** Optional wire fields are omitted for the empty case; the UI type keeps null. */
function toCareEvent(event: CareEventWire): CareEvent {
  switch (event.type) {
    case 'feed':
      return {
        id: event.id,
        member_id: event.member_id,
        type: 'feed',
        source: event.source,
        started_at: event.started_at,
        ended_at: event.ended_at,
        note: event.note,
        method: event.method,
        amount_ml: event.amount_ml ?? null,
        duration_min: event.duration_min ?? null,
      };
    case 'diaper':
      return {
        id: event.id,
        member_id: event.member_id,
        type: 'diaper',
        source: event.source,
        started_at: event.started_at,
        ended_at: event.ended_at,
        note: event.note,
        diaper_type: event.diaper_type,
      };
    case 'sleep':
      return {
        id: event.id,
        member_id: event.member_id,
        type: 'sleep',
        source: event.source,
        started_at: event.started_at,
        ended_at: event.ended_at,
        note: event.note,
        duration_min: event.duration_min ?? null,
      };
    case 'solid':
      return {
        id: event.id,
        member_id: event.member_id,
        type: 'solid',
        source: event.source,
        started_at: event.started_at,
        ended_at: event.ended_at,
        note: event.note,
        food: event.food,
        amount_g: event.amount_g ?? null,
      };
  }
}
