import { render, screen } from '@testing-library/react-native';

import type { CurrentChildState } from '../members/currentChild';
import { makeMember } from '../test/fixtures';
import { ChildSwitcher } from './ChildSwitcher';

const mockFocus = { focused: false };

jest.mock('expo-router', () => {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const React = require('react');
  return {
    router: { push: jest.fn() },
    useFocusEffect: (effect: () => void | (() => void)) => {
      // Stand-in for navigation focus: run the effect only while focused.
      React.useEffect(() => {
        if (!mockFocus.focused) {
          return undefined;
        }
        return effect();
        // The mock's focus flag is external state; rerendering is how the test moves focus.
        // eslint-disable-next-line react-hooks/exhaustive-deps
      }, [effect, mockFocus.focused]);
    },
  };
});

const members = [
  makeMember({ member_id: 1, name: '豆豆', is_child: true }),
  makeMember({ member_id: 2, name: '老二', is_child: true }),
];

const needsSelection: CurrentChildState = { status: 'needs-selection' };

describe('ChildSwitcher', () => {
  it('does not auto-open the picker while the daily screen is blurred', async () => {
    mockFocus.focused = false;

    const { rerender } = await render(
      <ChildSwitcher childState={needsSelection} members={members} onSelect={jest.fn()} />,
    );
    expect(screen.queryByTestId('child-picker')).toBeNull();

    mockFocus.focused = true;
    await rerender(
      <ChildSwitcher childState={needsSelection} members={members} onSelect={jest.fn()} />,
    );
    expect(await screen.findByTestId('child-picker')).toBeTruthy();
  });
});
