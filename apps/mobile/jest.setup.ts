// Shared Jest doubles for screen-level tests. Individual tests mock the API
// client at the resolveApiClient seam; router/storage/native shells are inert.
jest.mock('expo-router', () => {
  const actual = jest.requireActual('expo-router');
  return { ...actual, ...require('./src/test/routerMock').routerModule };
});

jest.mock('expo-router/js-tabs', () => ({
  useBottomTabBarHeight: () => 0,
}));

jest.mock('@react-native-async-storage/async-storage', () =>
  require('@react-native-async-storage/async-storage/jest/async-storage-mock'),
);

jest.mock('react-native-safe-area-context', () => {
  const React = require('react');
  const { View } = require('react-native');
  const SafeArea = ({ children, ...props }: { children?: React.ReactNode }) =>
    React.createElement(View, props, children);
  return {
    SafeAreaProvider: SafeArea,
    SafeAreaView: SafeArea,
    useSafeAreaInsets: () => ({ top: 0, right: 0, bottom: 0, left: 0 }),
    initialWindowMetrics: {
      frame: { x: 0, y: 0, width: 390, height: 844 },
      insets: { top: 0, right: 0, bottom: 0, left: 0 },
    },
  };
});
