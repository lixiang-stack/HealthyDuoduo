// B3 navigation vocabulary + pure route memory. Two domains, at most two levels:
// domain tabs live in the appbar; each domain owns its contextual bottom tabs.
// Stack pages (report detail, today records, reminder) stay outside these routes.

export const DOMAINS = ['care', 'health'] as const;
export type Domain = (typeof DOMAINS)[number];

export const CARE_PATHS = ['/care/today', '/care/stats'] as const;
export const HEALTH_PATHS = ['/health/reports', '/health/trends'] as const;

export type CarePath = (typeof CARE_PATHS)[number];
export type HealthPath = (typeof HEALTH_PATHS)[number];
export type DomainPath = CarePath | HealthPath;

export type DomainPaths = {
  care: CarePath;
  health: HealthPath;
};

export type DomainNavigationState = {
  /** Missing until the first domain visit; drives the remembered cold-start entry. */
  lastDomain?: Domain;
  lastPaths: DomainPaths;
};

export const DOMAIN_ROOT: Record<Domain, DomainPath> = {
  care: '/care/today',
  health: '/health/reports',
};

export const DEFAULT_DOMAIN_NAVIGATION: DomainNavigationState = {
  lastPaths: { care: '/care/today', health: '/health/reports' },
};

export function isCarePath(value: unknown): value is CarePath {
  return typeof value === 'string' && CARE_PATHS.includes(value as CarePath);
}

export function isHealthPath(value: unknown): value is HealthPath {
  return typeof value === 'string' && HEALTH_PATHS.includes(value as HealthPath);
}

export function domainOfPath(pathname: string): Domain | undefined {
  if (isCarePath(pathname)) {
    return 'care';
  }
  if (isHealthPath(pathname)) {
    return 'health';
  }
  return undefined;
}

/** Record a domain view; stack routes and unknown paths leave the state untouched. */
export function rememberDomainPath(
  state: DomainNavigationState,
  pathname: string,
): DomainNavigationState {
  const domain = domainOfPath(pathname);
  if (domain === undefined) {
    return state;
  }
  if (state.lastDomain === domain && state.lastPaths[domain] === pathname) {
    return state;
  }
  if (domain === 'care') {
    return {
      lastDomain: domain,
      lastPaths: { ...state.lastPaths, care: pathname as CarePath },
    };
  }
  return {
    lastDomain: domain,
    lastPaths: { ...state.lastPaths, health: pathname as HealthPath },
  };
}

/** Appbar tab semantics: active domain returns to its root, other domain resumes its last view. */
export function domainTabTarget(
  state: DomainNavigationState,
  activeDomain: Domain,
  pressedDomain: Domain,
): DomainPath {
  return activeDomain === pressedDomain ? DOMAIN_ROOT[pressedDomain] : state.lastPaths[pressedDomain];
}

export function defaultDomain(hasChild: boolean): Domain {
  return hasChild ? 'care' : 'health';
}

/** First launch falls back by child presence; later launches resume the remembered view. */
export function resolveEntryPath(
  state: DomainNavigationState,
  hasChild: boolean,
): DomainPath {
  return state.lastDomain === undefined
    ? DOMAIN_ROOT[defaultDomain(hasChild)]
    : state.lastPaths[state.lastDomain];
}
