// HTTP care-event repository (spec §7.2). Every call re-resolves the configured
// backend so a Base URL change in settings takes effect without an app restart.
// There is no local cache / optimistic update / offline queue: failures surface to
// the existing loading and error states instead of silently falling back in memory.
import {
  createCareEvent,
  deleteCareEvent,
  listCareEvents,
  updateCareEvent,
} from '../api/careEvents';
import { resolveApiClient } from '../api/resolveClient';
import type {
  CareEvent,
  CareEventCreate,
  CareEventQuery,
  CareEventRepository,
  CareEventUpdate,
} from './careEvents';

export const CARE_EVENTS_UNCONFIGURED_MESSAGE = '尚未配置后端地址';

export class HttpCareEventRepository implements CareEventRepository {
  constructor(private readonly resolveClient = resolveApiClient) {}

  async list(query: CareEventQuery): Promise<CareEvent[]> {
    return listCareEvents(await this.client(), query.member_id, {
      from: query.from,
      to: query.to,
    });
  }

  async create(event: CareEventCreate): Promise<CareEvent> {
    return createCareEvent(await this.client(), event.member_id, event);
  }

  async update(eventId: number, event: CareEventUpdate): Promise<CareEvent> {
    return updateCareEvent(await this.client(), eventId, event);
  }

  async remove(eventId: number): Promise<void> {
    await deleteCareEvent(await this.client(), eventId);
  }

  private async client() {
    const resolved = await this.resolveClient();
    if (!resolved) {
      throw new Error(CARE_EVENTS_UNCONFIGURED_MESSAGE);
    }
    return resolved.client;
  }
}
