import { render, type RenderOptions } from '@testing-library/react-native';
import type { ReactElement } from 'react';

import { ImageViewerProvider } from '../components/imageViewer/ImageViewerProvider';
import { CurrentMemberProvider } from '../members/CurrentMemberContext';

/** Render a screen with app-level member and image viewer contexts. AsyncStorage
 * is the shared Jest mock, so a test can persist a current member before rendering. */
export function renderScreen(ui: ReactElement, options?: RenderOptions) {
  return render(ui, {
    wrapper: ({ children }) => (
      <CurrentMemberProvider>
        <ImageViewerProvider>{children}</ImageViewerProvider>
      </CurrentMemberProvider>
    ),
    ...options,
  });
}
