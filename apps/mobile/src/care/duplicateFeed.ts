// Duplicate-feed guard (ticket 05): before saving a feed, look for another feed
// for the same child within five minutes. This is a best-effort client check:
// two devices can still both save inside the window (LAN-first, no accounts), and
// the server never deduplicates.
import { compareCareEventsDesc, type CareEvent, type CareEventRange } from './careEvents';

export const DUPLICATE_FEED_WINDOW_MINUTES = 5;
export const DUPLICATE_FEED_WINDOW_MS = DUPLICATE_FEED_WINDOW_MINUTES * 60000;

export type DuplicateFeedQuery = {
  memberId: number;
  startedAt: string;
  /** Editing this feed must not match itself. */
  editingEventId?: number;
};

/** Inclusive window around the candidate time, widened by 1 ms so a feed exactly
 *  at the right edge still passes the repository's exclusive `to`. */
export function duplicateFeedQueryRange(startedAt: string): CareEventRange {
  const started = Date.parse(startedAt);
  return {
    from: new Date(started - DUPLICATE_FEED_WINDOW_MS).toISOString(),
    to: new Date(started + DUPLICATE_FEED_WINDOW_MS + 1).toISOString(),
  };
}

export function findDuplicateFeed(
  events: CareEvent[],
  query: DuplicateFeedQuery,
): CareEvent | undefined {
  const target = Date.parse(query.startedAt);
  return events
    .filter(
      (event) =>
        event.type === 'feed' &&
        event.member_id === query.memberId &&
        event.id !== query.editingEventId &&
        Math.abs(Date.parse(event.started_at) - target) <= DUPLICATE_FEED_WINDOW_MS,
    )
    .sort(compareCareEventsDesc)[0];
}
