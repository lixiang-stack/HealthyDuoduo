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

function Link({ asChild, children, ...props }: { asChild?: boolean; children: React.ReactNode }) {
  if (asChild) {
    return React.createElement(React.Fragment, null, children);
  }
  return React.createElement(Text, props, children);
}

function useFocusEffect(effect: React.EffectCallback) {
  React.useEffect(effect, [effect]);
}

export const routerModule = {
  router: mockRouter,
  Link,
  useLocalSearchParams: mockUseLocalSearchParams,
  useFocusEffect,
};
