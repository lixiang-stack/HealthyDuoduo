// Shared hook used by the care view: the device-local reminder always follows the
// current child. DailyPlaceholder calls it for both the today and stats views, so a
// child switch re-arms the schedule no matter which view is mounted. A failed read on
// a switch stays cancelled (fail closed); a failed cold-start read keeps the existing
// notification until the next successful load corrects it (spec §7.5).
import { useEffect, useRef } from 'react';

import { useCareEventRepository } from '../care/CareEventProvider';
import { syncReminderToLatestFeed } from './syncReminderToLatestFeed';

export function useReminderResync(
  childId: number | undefined,
  careReady: boolean,
): void {
  const repository = useCareEventRepository();
  const previousChildId = useRef<number | undefined>(undefined);

  useEffect(() => {
    if (!careReady) {
      return;
    }
    const switched =
      previousChildId.current !== undefined && previousChildId.current !== childId;
    previousChildId.current = childId;
    void syncReminderToLatestFeed({
      repository,
      memberId: childId,
      cancelBeforeRead: switched,
    }).catch(() => undefined);
  }, [careReady, childId, repository]);
}
