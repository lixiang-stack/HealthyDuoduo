import { render, screen } from '@testing-library/react-native';

import DailyTabsLayout from '../../app/daily/_layout';
import HealthTabsLayout from '../../app/health/_layout';

jest.mock('../../navigation/DomainHeader', () => ({
  DomainHeader: () => null,
}));

jest.mock('expo-router/js-tabs', () => {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const React = require('react');
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const { Text } = require('react-native');
  const Tabs = ({ children }: { children: unknown }) =>
    React.createElement(React.Fragment, null, children);
  function Screen({
    options,
  }: {
    options?: { title?: string; tabBarButtonTestID?: string };
  }) {
    return React.createElement(Text, { testID: options?.tabBarButtonTestID }, options?.title);
  }
  Tabs.Screen = Screen;
  return { Tabs, useBottomTabBarHeight: () => 0 };
});

describe('domain tab layouts', () => {
  it('daily renders only today and stats', async () => {
    await render(<DailyTabsLayout />);

    expect(screen.getByTestId('daily-tab-today')).toHaveTextContent('今天');
    expect(screen.getByTestId('daily-tab-stats')).toHaveTextContent('统计');
    expect(screen.queryByTestId('health-tab-reports')).toBeNull();
    expect(screen.queryByTestId('health-tab-trends')).toBeNull();
  });

  it('health renders only reports and trends', async () => {
    await render(<HealthTabsLayout />);

    expect(screen.getByTestId('health-tab-reports')).toHaveTextContent('报告');
    expect(screen.getByTestId('health-tab-trends')).toHaveTextContent('趋势');
    expect(screen.queryByTestId('daily-tab-today')).toBeNull();
    expect(screen.queryByTestId('daily-tab-stats')).toBeNull();
  });
});
