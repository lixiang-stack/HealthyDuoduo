import { fireEvent, render, screen } from '@testing-library/react-native';

import { colors } from '../components/theme';
import { DomainTabs } from './DomainTabs';

describe('DomainTabs', () => {
  it('labels the two domains as 育儿 and 健康档案', async () => {
    await render(<DomainTabs activeDomain="daily" hasPending={false} onSelect={jest.fn()} />);

    expect(screen.getByTestId('domain-tab-daily-label')).toHaveTextContent('育儿');
    expect(screen.getByTestId('domain-tab-health-label')).toHaveTextContent('健康档案');
  });

  it('marks the active domain with bold text and a 2px underline', async () => {
    await render(<DomainTabs activeDomain="daily" hasPending onSelect={jest.fn()} />);

    expect(screen.getByTestId('domain-tab-daily').props.accessibilityState).toEqual({
      selected: true,
    });
    expect(screen.getByTestId('domain-tab-daily-label')).toHaveStyle({ fontWeight: '600' });
    expect(screen.getByTestId('domain-tab-daily-underline')).toHaveStyle({
      height: 2,
      backgroundColor: colors.accent,
    });
    expect(screen.getByTestId('domain-tab-health-underline')).toHaveStyle({
      backgroundColor: 'transparent',
    });
  });

  it('shows the health pending dot only when there is an actionable count', async () => {
    const { rerender } = await render(
      <DomainTabs activeDomain="daily" hasPending={false} onSelect={jest.fn()} />,
    );
    expect(screen.queryByTestId('domain-tab-health-dot')).toBeNull();

    await rerender(<DomainTabs activeDomain="daily" hasPending onSelect={jest.fn()} />);
    expect(screen.getByTestId('domain-tab-health-dot')).toBeTruthy();
  });

  it('reports domain presses so the shell can preserve secondary state', async () => {
    const onSelect = jest.fn();
    await render(<DomainTabs activeDomain="daily" hasPending={false} onSelect={onSelect} />);

    fireEvent.press(screen.getByTestId('domain-tab-daily'));
    fireEvent.press(screen.getByTestId('domain-tab-health'));

    expect(onSelect).toHaveBeenNthCalledWith(1, 'daily');
    expect(onSelect).toHaveBeenNthCalledWith(2, 'health');
  });
});
