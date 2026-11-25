// App-level care-event repository seam. The root layout injects the HTTP repository;
// tests inject an in-memory one. There is deliberately no default: forgetting to wire
// the provider is a type error instead of silently showing an empty in-memory log.
import { createContext, useContext, type ReactNode } from 'react';

import type { CareEventRepository } from './careEvents';

const CareEventContext = createContext<CareEventRepository | undefined>(undefined);

export function CareEventProvider({
  repository,
  children,
}: {
  repository: CareEventRepository;
  children: ReactNode;
}) {
  return <CareEventContext.Provider value={repository}>{children}</CareEventContext.Provider>;
}

export function useCareEventRepository(): CareEventRepository {
  const value = useContext(CareEventContext);
  if (!value) {
    throw new Error('useCareEventRepository must be used inside CareEventProvider');
  }
  return value;
}
