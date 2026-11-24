// Seven-day care-event read model for the stats page. Kept separate from today's
// timeline so switching to stats does not depend on the local-day filtering.
import { useFocusEffect } from 'expo-router';
import { useCallback, useRef, useState } from 'react';

import { useCareEventRepository } from './CareEventProvider';
import { careStatsRange } from './careStats';
import type { CareEvent } from './careEvents';

export type StatsCareEventsState =
  | { status: 'loading' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; events: CareEvent[] };

export function useStatsCareEvents(memberId: number): {
  state: StatsCareEventsState;
  reload: () => Promise<void>;
} {
  const repository = useCareEventRepository();
  const [state, setState] = useState<StatsCareEventsState>({ status: 'loading' });
  const requestId = useRef(0);

  const reload = useCallback(async () => {
    const request = ++requestId.current;
    try {
      const events = await repository.list({
        member_id: memberId,
        ...careStatsRange(new Date()),
      });
      if (request === requestId.current) {
        setState({ status: 'ready', events });
      }
    } catch (error) {
      if (request === requestId.current) {
        setState({
          status: 'error',
          detail: error instanceof Error ? error.message : String(error),
        });
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
