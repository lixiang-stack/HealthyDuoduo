import { makeFeedEvent } from '../test/fixtures';
import type { CareEvent } from './careEvents';
import { duplicateFeedQueryRange, findDuplicateFeed } from './duplicateFeed';

const STARTED_AT = new Date(2026, 10, 23, 14, 5).toISOString();

function minutesFrom(startedAt: string, minutes: number): string {
  return new Date(Date.parse(startedAt) + minutes * 60000).toISOString();
}

describe('duplicate feed window', () => {
  it('queries an inclusive window around the candidate time', () => {
    const range = duplicateFeedQueryRange(STARTED_AT);

    expect(Date.parse(range.from)).toBe(Date.parse(STARTED_AT) - 5 * 60000);
    expect(Date.parse(range.to)).toBe(Date.parse(STARTED_AT) + 5 * 60000 + 1);
  });

  it('triggers at the inclusive five-minute boundary', () => {
    const exactlyFive = makeFeedEvent({ started_at: minutesFrom(STARTED_AT, -5) });

    expect(
      findDuplicateFeed([exactlyFive], { memberId: 1, startedAt: STARTED_AT })?.id,
    ).toBe(1);
  });

  it('triggers just outside the window when the event is one millisecond further away', () => {
    const justOutside = makeFeedEvent({
      id: 2,
      started_at: new Date(Date.parse(STARTED_AT) - 5 * 60000 - 1).toISOString(),
    });

    expect(findDuplicateFeed([justOutside], { memberId: 1, startedAt: STARTED_AT })).toBeUndefined();
  });

  it('finds a previous feed just before midnight', () => {
    const candidate = new Date(2026, 10, 24, 0, 2).toISOString();
    const previous = makeFeedEvent({
      id: 2,
      started_at: new Date(2026, 10, 23, 23, 58).toISOString(),
    });
    const range = duplicateFeedQueryRange(candidate);

    expect(Date.parse(range.from)).toBeLessThanOrEqual(Date.parse(previous.started_at));
    expect(Date.parse(range.to)).toBeGreaterThan(Date.parse(previous.started_at));
    expect(
      findDuplicateFeed([previous], { memberId: 1, startedAt: candidate })?.id,
    ).toBe(2);
  });

  it('ignores other children, other event types and the event being edited', () => {
    const otherChild = makeFeedEvent({ id: 2, member_id: 2, started_at: STARTED_AT });
    const diaper: CareEvent = {
      id: 3,
      member_id: 1,
      type: 'diaper',
      started_at: STARTED_AT,
      ended_at: null,
      note: null,
      source: 'parent',
      diaper_type: 'wet',
    };
    const editing = makeFeedEvent({ id: 4, started_at: STARTED_AT });

    expect(
      findDuplicateFeed([otherChild, diaper], { memberId: 1, startedAt: STARTED_AT }),
    ).toBeUndefined();
    expect(
      findDuplicateFeed([editing], { memberId: 1, startedAt: STARTED_AT, editingEventId: 4 }),
    ).toBeUndefined();
  });

  it('still triggers for another feed while editing a different event', () => {
    const editing = makeFeedEvent({ id: 4, started_at: STARTED_AT });
    const other = makeFeedEvent({ id: 5, started_at: minutesFrom(STARTED_AT, -1) });

    expect(
      findDuplicateFeed([editing, other], { memberId: 1, startedAt: STARTED_AT, editingEventId: 4 })
        ?.id,
    ).toBe(5);
  });

  it('returns the newest candidate in the window', () => {
    const older = makeFeedEvent({ id: 1, started_at: minutesFrom(STARTED_AT, -4) });
    const newer = makeFeedEvent({ id: 2, started_at: minutesFrom(STARTED_AT, -1) });

    expect(findDuplicateFeed([older, newer], { memberId: 1, startedAt: STARTED_AT })?.id).toBe(2);
  });
});
