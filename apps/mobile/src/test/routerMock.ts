import * as React from 'react';
import { Text } from 'react-native';

/** Shared Expo Router doubles for screen tests. Kept outside `src/app` so
 * Metro never treats this file as a route. */
export const mockRouter = {
  push: jest.fn(),
  replace: jest.fn(),
  back: jest.fn(),
  dismissTo: jest.fn(),
};

export const mockUseLocalSearchParams = jest.fn<Record<string, string>, []>(() => ({}));

/** Mutable current pathname for tests that render navigation-aware containers. */
export const mockPathname = { current: '/' };

function Link({ asChild, children, ...props }: { asChild?: boolean; children: React.ReactNode }) {
  if (asChild) {
    return React.createElement(React.Fragment, null, children);
  }
  return React.createElement(Text, props, children);
}

/** Renders the target path as text so entry-route tests can assert where it goes. */
function Redirect({ href }: { href: string | { pathname: string } }) {
  return React.createElement(
    Text,
    { testID: 'redirect' },
    typeof href === 'string' ? href : href.pathname,
  );
}

function useFocusEffect(effect: React.EffectCallback) {
  React.useEffect(effect, [effect]);
}

function usePathname() {
  return mockPathname.current;
}

/** Screen-level tests render a route component directly; the navigator's
 * `Stack.Screen` options setter is inert without a route tree. */
function Screen() {
  return null;
}

export const routerModule = {
  router: mockRouter,
  Link,
  Redirect,
  useLocalSearchParams: mockUseLocalSearchParams,
  useFocusEffect,
  usePathname,
  Stack: { Screen },
};
