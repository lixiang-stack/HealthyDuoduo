// App-level care-event repository seam. Tickets 04 / 06 / 07 consume the same
// in-memory adapter until the HTTP adapter replaces it; keeping it in context makes
// the dependency explicit and lets tests inject a repository with real fixtures.
import { createContext, useContext, useMemo, type ReactNode } from 'react';

import { CARE_EVENT_FIXTURES } from './careEventFixtures';
import { InMemoryCareEventRepository } from './careEventRepository';
import type { CareEventRepository } from './careEvents';

const CareEventContext = createContext<CareEventRepository | undefined>(undefined);

export function CareEventProvider({
  repository,
  children,
}: {
  repository?: CareEventRepository;
  children: ReactNode;
}) {
  const value = useMemo(
    () => repository ?? new InMemoryCareEventRepository(CARE_EVENT_FIXTURES),
    [repository],
  );
  return <CareEventContext.Provider value={value}>{children}</CareEventContext.Provider>;
}

export function useCareEventRepository(): CareEventRepository {
  const value = useContext(CareEventContext);
  if (!value) {
    throw new Error('useCareEventRepository must be used inside CareEventProvider');
  }
  return value;
}
