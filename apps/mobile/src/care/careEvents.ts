// Client-visible care-event contract (ticket 01). This is intentionally narrower than
// the eventual table/API spec: it freezes only the shape the daily timeline, stats and
// duplicate-feed warning need. See docs/design/2026-11-23-care-event-contract.md.
//
// Care events always target one child member and are self-reported by a parent.
// There is deliberately no caregiver, device or account attribution field, and no
// global active-child field: LAN-first, no accounts (ADR-0006).

export const CARE_EVENT_TYPES = ['feed', 'diaper', 'sleep', 'solid'] as const;
export type CareEventType = (typeof CARE_EVENT_TYPES)[number];

export const FEED_METHODS = ['breast', 'formula'] as const;
export type FeedMethod = (typeof FEED_METHODS)[number];

export const DIAPER_TYPES = ['wet', 'dirty', 'mixed'] as const;
export type DiaperType = (typeof DIAPER_TYPES)[number];

export const CARE_EVENT_SOURCE_PARENT = 'parent' as const;
export type CareEventSource = typeof CARE_EVENT_SOURCE_PARENT;

type CareEventBodyBase = {
  /** Absolute ISO 8601 instant with an offset (cross-midnight stays on one event). */
  started_at: string;
  /** Null while sleep is ongoing. Feed / diaper / solid events are point events at
   *  `started_at`, so they also store null. */
  ended_at: string | null;
  /** Trimmed free text; null when empty. */
  note: string | null;
};

export type FeedCareEventBody = CareEventBodyBase & {
  type: 'feed';
  method: FeedMethod;
  /** Bottle/formula amount; null for unscheduled breastfeeding. */
  amount_ml: number | null;
  /** Breastfeeding duration; null when only the start was recorded. */
  duration_min: number | null;
};

export type DiaperCareEventBody = CareEventBodyBase & {
  type: 'diaper';
  diaper_type: DiaperType;
};

export type SleepCareEventBody = CareEventBodyBase & {
  type: 'sleep';
  /** Null while sleep is ongoing. */
  duration_min: number | null;
};

export type SolidCareEventBody = CareEventBodyBase & {
  type: 'solid';
  /** Required non-empty food label, e.g. 米粉. */
  food: string;
  amount_g: number | null;
};

export type CareEventBody =
  | FeedCareEventBody
  | DiaperCareEventBody
  | SleepCareEventBody
  | SolidCareEventBody;

export type CareEvent = CareEventBody & {
  id: number;
  /** Required owner; events never belong to an adult member. */
  member_id: number;
  source: CareEventSource;
};

/** Create payload: the owner is explicit, identity/source are assigned by storage. */
export type CareEventCreate = CareEventBody & { member_id: number };

/** Update payload replaces the editable content; identity, owner and source stay fixed. */
export type CareEventUpdate = CareEventBody;

/** Inclusive `from`, exclusive `to`, both absolute ISO 8601 instants. */
export type CareEventRange = {
  from: string;
  to: string;
};

export type CareEventQuery = CareEventRange & { member_id: number };

/** Read/write seam consumed by tickets 04 / 06 / 07 until the HTTP adapter exists. */
export type CareEventRepository = {
  list(query: CareEventQuery): Promise<CareEvent[]>;
  create(event: CareEventCreate): Promise<CareEvent>;
  update(eventId: number, event: CareEventUpdate): Promise<CareEvent>;
  remove(eventId: number): Promise<void>;
};

/** Range reads are overlap-based so a sleep that starts before midnight still appears
 *  in today's timeline. `from` is inclusive and `to` is exclusive. A `null` end means
 *  an ongoing sleep for sleep events, but a point event for feed / diaper / solid. */
export function careEventOverlapsRange(
  event: Pick<CareEvent, 'type' | 'started_at' | 'ended_at'>,
  range: CareEventRange,
): boolean {
  const started = Date.parse(event.started_at);
  const from = Date.parse(range.from);
  const to = Date.parse(range.to);
  if (started >= to) {
    return false;
  }
  if (event.ended_at === null) {
    if (event.type === 'sleep') {
      // Ongoing sleep has an open end, so any earlier range still intersects it.
      return true;
    }
    // Point events are half-open [start, start+1): include the inclusive `from` edge.
    return started >= from;
  }
  const ended = Date.parse(event.ended_at);
  if (ended === started) {
    // Point events are half-open [start, start+1): include the inclusive `from` edge.
    return started >= from;
  }
  return ended > from;
}

/** Newest first; id breaks ties for deterministic timelines. */
export function compareCareEventsDesc(a: CareEvent, b: CareEvent): number {
  const byStart = Date.parse(b.started_at) - Date.parse(a.started_at);
  return byStart !== 0 ? byStart : b.id - a.id;
}

/** Sleep duration from the stored minutes or from a completed start/end pair. */
export function careEventDurationMinutes(event: CareEvent): number | null {
  if (event.type !== 'sleep') {
    return null;
  }
  if (event.duration_min !== null) {
    return event.duration_min;
  }
  if (event.ended_at === null) {
    return null;
  }
  return Math.max(0, Math.round((Date.parse(event.ended_at) - Date.parse(event.started_at)) / 60000));
}
