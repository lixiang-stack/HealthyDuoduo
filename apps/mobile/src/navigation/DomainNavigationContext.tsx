// Device-local B3 domain navigation: remembers each domain's last secondary view and
// exposes replace-based switching so domain changes never grow the back stack.
import { router, usePathname } from 'expo-router';
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

import {
  DEFAULT_DOMAIN_NAVIGATION,
  domainOfPath,
  domainTabTarget,
  rememberDomainPath,
  type Domain,
  type DomainNavigationState,
} from './domainRoutes';
import { loadDomainNavigation, saveDomainNavigation } from './domainNavigationStore';

type DomainNavigationContextValue = {
  state: DomainNavigationState;
  navigateFromDomain: (activeDomain: Domain, pressedDomain: Domain) => void;
};

const DomainNavigationContext = createContext<DomainNavigationContextValue | undefined>(
  undefined,
);

export function DomainNavigationProvider({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const [remembered, setRemembered] =
    useState<DomainNavigationState>(DEFAULT_DOMAIN_NAVIGATION);
  const [ready, setReady] = useState(false);
  // Derive the current snapshot instead of writing pathname changes back through an
  // effect: the provider re-renders on every pathname change, so bottom-tab presses are
  // captured here and persisted without a second cascading render.
  const state = rememberDomainPath(remembered, pathname);

  useEffect(() => {
    let cancelled = false;
    void loadDomainNavigation().then((stored) => {
      if (!cancelled) {
        setRemembered(stored);
        setReady(true);
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    // Stack pages keep the domain header hidden; do not overwrite the remembered tab
    // with a fallback snapshot while a detail page is open.
    if (!ready || domainOfPath(pathname) === undefined) {
      return;
    }
    void saveDomainNavigation(state);
  }, [ready, state, pathname]);

  const navigateFromDomain = useCallback(
    (activeDomain: Domain, pressedDomain: Domain) => {
      setRemembered(state);
      router.replace(domainTabTarget(state, activeDomain, pressedDomain));
    },
    [state],
  );

  const value = useMemo(
    () => ({ state, navigateFromDomain }),
    [state, navigateFromDomain],
  );

  return (
    <DomainNavigationContext.Provider value={value}>
      {children}
    </DomainNavigationContext.Provider>
  );
}

export function useDomainNavigation(): DomainNavigationContextValue {
  const value = useContext(DomainNavigationContext);
  if (!value) {
    throw new Error('useDomainNavigation must be used inside DomainNavigationProvider');
  }
  return value;
}
