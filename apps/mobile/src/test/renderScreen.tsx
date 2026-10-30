import { render, type RenderOptions } from '@testing-library/react-native';
import type { ReactElement } from 'react';

import { CurrentMemberProvider } from '../members/CurrentMemberContext';

/** Render a screen with the app-level member context. AsyncStorage is the
 * shared Jest mock, so a test can persist a current member before rendering. */
export function renderScreen(ui: ReactElement, options?: RenderOptions) {
  return render(ui, {
    wrapper: ({ children }) => <CurrentMemberProvider>{children}</CurrentMemberProvider>,
    ...options,
  });
}
