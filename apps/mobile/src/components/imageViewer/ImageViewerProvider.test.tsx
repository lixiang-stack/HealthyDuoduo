import { fireEvent, render, waitFor } from '@testing-library/react-native';
import { Image, Pressable, Text } from 'react-native';

import { ImageViewerProvider, useImageViewer } from './ImageViewerProvider';

function Host() {
  const { open } = useImageViewer();
  return (
    <Pressable testID="open-viewer" onPress={() => open('http://test/check.jpg', '检查单原图')}>
      <Text>open</Text>
    </Pressable>
  );
}

describe('ImageViewerProvider', () => {
  beforeEach(() => {
    jest
      .spyOn(Image, 'getSize')
      .mockImplementation(((_url: string, success: (width: number, height: number) => void) =>
        success(1000, 1500)) as typeof Image.getSize);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('opens the full-screen viewer and closes it', async () => {
    const view = await render(
      <ImageViewerProvider>
        <Host />
      </ImageViewerProvider>,
    );

    fireEvent.press(view.getByTestId('open-viewer'));
    const root = await view.findByTestId('image-viewer-root');
    fireEvent(root, 'layout', {
      nativeEvent: { layout: { width: 390, height: 844 } },
    });

    expect(await view.findByTestId('image-viewer-image')).toBeTruthy();
    expect(view.getByTestId('image-viewer-reset')).toBeTruthy();

    fireEvent.press(view.getByTestId('image-viewer-close'));
    await waitFor(() => expect(view.queryByTestId('image-viewer-image')).toBeNull());
  });
});
