// Keep the local reminder aligned with the latest feed of one child. Called after a
// record is created / edited / deleted; mount / child switches do not touch the OS
// schedule (that is outside ticket 06 and avoids cancelling on a cold start).
import { latestFeed, recentCareEventRange } from '../care/careEventDisplay';
import type { CareEventRepository } from '../care/careEvents';
import { syncReminderNotification } from './reminderNotifications';
import { loadReminderSettings } from './reminderSettings';

export async function syncReminderToLatestFeed({
  repository,
  memberId,
  now = new Date(),
}: {
  repository: CareEventRepository;
  memberId: number;
  now?: Date;
}): Promise<void> {
  const settings = await loadReminderSettings();
  if (!settings.enabled) {
    return;
  }
  const recent = await repository.list({ member_id: memberId, ...recentCareEventRange(now) });
  await syncReminderNotification(settings, latestFeed(recent, now)?.started_at, now);
}
