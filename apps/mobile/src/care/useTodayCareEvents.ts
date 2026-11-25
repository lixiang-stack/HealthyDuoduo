// Today's care events for the selected child. The read window also covers yesterday
// so the hero can keep showing the last feed just after midnight; the timeline itself
// is filtered back to the device-local day. A failed refresh keeps the previous data
// on screen and exposes the error alongside it.
import { useFocusEffect } from 'expo-router';
import { useCallback, useRef, useState } from 'react';

import { useCareEventRepository } from './CareEventProvider';
import {
  recentCareEventRange,
  splitTodayCareEvents,
} from './careEventDisplay';
import type { CareEvent } from './careEvents';

export type TodayCareEventsState =
  | { status: 'loading' }
  | { status: 'error'; detail: string }
  | {
      status: 'ready';
      events: CareEvent[];
      lastFeed: CareEvent | undefined;
      error?: string;
    };

export function useTodayCareEvents(memberId: number): {
  state: TodayCareEventsState;
  reload: () => Promise<void>;
} {
  const repository = useCareEventRepository();
  const [state, setState] = useState<TodayCareEventsState>({ status: 'loading' });
  const requestId = useRef(0);

  const reload = useCallback(async () => {
    const request = ++requestId.current;
    const now = new Date();
    try {
      const recent = await repository.list({
        member_id: memberId,
        ...recentCareEventRange(now),
      });
      if (request === requestId.current) {
        const { today, lastFeed } = splitTodayCareEvents(recent, now);
        setState({ status: 'ready', events: today, lastFeed });
      }
    } catch (error) {
      if (request === requestId.current) {
        const detail = error instanceof Error ? error.message : String(error);
        setState((previous) =>
          previous.status === 'ready' ? { ...previous, error: detail } : { status: 'error', detail },
        );
      }
    }
  }, [memberId, repository]);

  useFocusEffect(
    useCallback(() => {
      void reload();
    }, [reload]),
  );

  return { state, reload };
}
