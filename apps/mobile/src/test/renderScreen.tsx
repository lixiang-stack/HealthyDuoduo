import { render, type RenderOptions } from '@testing-library/react-native';
import type { ReactElement } from 'react';

import { CareEventProvider } from '../care/CareEventProvider';
import { InMemoryCareEventRepository } from '../care/careEventRepository';
import type { CareEventRepository } from '../care/careEvents';
import { ImageViewerProvider } from '../components/imageViewer/ImageViewerProvider';
import { CurrentMemberProvider } from '../members/CurrentMemberContext';

export type ScreenRenderOptions = RenderOptions & {
  /** Seed or spy on the care-event repository; defaults to an empty repository. */
  careEvents?: CareEventRepository;
};

/** Render a screen with app-level member, care-event and image viewer contexts.
 * AsyncStorage is the shared Jest mock, so a test can persist a current member
 * before rendering. Screens default to empty care events so tests seed only what
 * the case needs (the app provider keeps the ticket-01 fixtures until the API lands). */
export function renderScreen(
  ui: ReactElement,
  { careEvents, ...options }: ScreenRenderOptions = {},
) {
  const repository = careEvents ?? new InMemoryCareEventRepository();
  return render(ui, {
    wrapper: ({ children }) => (
      <CurrentMemberProvider>
        <CareEventProvider repository={repository}>
          <ImageViewerProvider>{children}</ImageViewerProvider>
        </CareEventProvider>
      </CurrentMemberProvider>
    ),
    ...options,
  });
}
