// Keep the local reminder aligned with the current child's latest feed. Called after a
// record changes, when the care view first mounts (cold start, failed reads keep the
// existing schedule), and when the current child switches (cancel first, so a failed
// read can never leave the previous child's reminder armed).
//
// The sync is idempotent for the same child + latest feed: focus / re-render must not
// replace the OS schedule repeatedly. Settings stay one per device and follow whichever
// child the care view currently shows.
import { latestFeed, recentCareEventRange } from '../care/careEventDisplay';
import type { CareEventRepository } from '../care/careEvents';
import { cancelReminderNotifications, syncReminderNotification } from './reminderNotifications';
import { loadReminderSettings } from './reminderSettings';

/** Dedupe key per data source; production has one app-wide repository. */
const appliedKeys = new WeakMap<CareEventRepository, string>();
/** Latest sync per data source; a slower earlier read must not win a switch race. */
const requests = new WeakMap<CareEventRepository, number>();

export async function syncReminderToLatestFeed({
  repository,
  memberId,
  now = new Date(),
  cancelBeforeRead = false,
}: {
  repository: CareEventRepository;
  /** Undefined means the care view has no current child; the old schedule is cleared. */
  memberId: number | undefined;
  now?: Date;
  /** Child switch: cancel the old schedule before reading the new child. */
  cancelBeforeRead?: boolean;
}): Promise<void> {
  const request = (requests.get(repository) ?? 0) + 1;
  requests.set(repository, request);
  const isLatest = () => requests.get(repository) === request;

  if (memberId === undefined) {
    if (isLatest()) {
      appliedKeys.delete(repository);
      await cancelReminderNotifications();
    }
    return;
  }

  if (cancelBeforeRead) {
    // Fail closed: if the new child's read fails below, no old reminder may fire.
    await cancelReminderNotifications();
    appliedKeys.delete(repository);
  }
  if (!isLatest()) {
    return;
  }

  const settings = await loadReminderSettings();
  if (!isLatest()) {
    return;
  }
  if (!settings.enabled) {
    // A disabled reminder must not leave an earlier schedule armed.
    appliedKeys.delete(repository);
    await cancelReminderNotifications();
    return;
  }

  const recent = await repository.list({ member_id: memberId, ...recentCareEventRange(now) });
  if (!isLatest()) {
    return;
  }
  const lastFeed = latestFeed(recent, now);
  const key = `${memberId}:${lastFeed?.started_at ?? 'none'}:${settings.intervalHours}`;
  if (appliedKeys.get(repository) === key) {
    return;
  }

  await syncReminderNotification(settings, lastFeed?.started_at, now, isLatest);
  if (isLatest()) {
    appliedKeys.set(repository, key);
  }
}
