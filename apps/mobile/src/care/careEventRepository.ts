// In-memory adapter for the care-event contract. Tests and explicit previews inject
// it through CareEventProvider; production uses HttpCareEventRepository.
import {
  careEventOverlapsRange,
  compareCareEventsDesc,
  CARE_EVENT_SOURCE_PARENT,
  type CareEvent,
  type CareEventCreate,
  type CareEventQuery,
  type CareEventRepository,
  type CareEventUpdate,
} from './careEvents';

export class InMemoryCareEventRepository implements CareEventRepository {
  private events: CareEvent[];
  private nextId: number;

  constructor(seed: CareEvent[] = []) {
    this.events = seed.map((event) => ({ ...event }));
    this.nextId = seed.reduce((max, event) => Math.max(max, event.id), 0) + 1;
  }

  async list(query: CareEventQuery): Promise<CareEvent[]> {
    return this.events
      .filter(
        (event) =>
          event.member_id === query.member_id && careEventOverlapsRange(event, query),
      )
      .sort(compareCareEventsDesc);
  }

  async create(input: CareEventCreate): Promise<CareEvent> {
    const { member_id, ...body } = input;
    const event: CareEvent = {
      ...body,
      id: this.nextId,
      member_id,
      source: CARE_EVENT_SOURCE_PARENT,
    };
    this.nextId += 1;
    this.events.push(event);
    return event;
  }

  async update(eventId: number, input: CareEventUpdate): Promise<CareEvent> {
    const index = this.events.findIndex((event) => event.id === eventId);
    if (index < 0) {
      throw new Error(`care event ${eventId} not found`);
    }
    const existing = this.events[index];
    const updated: CareEvent = {
      ...input,
      id: existing.id,
      member_id: existing.member_id,
      source: existing.source,
    };
    this.events[index] = updated;
    return updated;
  }

  async remove(eventId: number): Promise<void> {
    const index = this.events.findIndex((event) => event.id === eventId);
    if (index < 0) {
      throw new Error(`care event ${eventId} not found`);
    }
    this.events.splice(index, 1);
  }
}
