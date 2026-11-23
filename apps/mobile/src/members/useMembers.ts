// Member list for navigation/child-context checks. Kept separate from report loading
// so daily placeholders do not fetch a member's reports just to know who is a child.
import { useFocusEffect } from 'expo-router';
import { useCallback, useRef, useState } from 'react';

import { listMembers, type Member } from '../api/members';
import { resolveApiClient } from '../api/resolveClient';

export type MembersState =
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; members: Member[] };

export function useMembers(): {
  state: MembersState;
  reload: () => Promise<void>;
} {
  const [state, setState] = useState<MembersState>({ status: 'loading' });
  const requestId = useRef(0);

  const reload = useCallback(async () => {
    const request = ++requestId.current;
    const resolved = await resolveApiClient();
    if (request !== requestId.current) {
      return;
    }
    if (!resolved) {
      setState({ status: 'unconfigured' });
      return;
    }
    try {
      const members = await listMembers(resolved.client);
      if (request !== requestId.current) {
        return;
      }
      setState({ status: 'ready', members });
    } catch (error) {
      if (request !== requestId.current) {
        return;
      }
      setState({ status: 'error', detail: error instanceof Error ? error.message : String(error) });
    }
  }, []);

  useFocusEffect(
    useCallback(() => {
      void reload();
    }, [reload]),
  );

  return { state, reload };
}
