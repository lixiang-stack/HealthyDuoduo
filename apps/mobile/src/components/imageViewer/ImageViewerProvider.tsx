// 全屏原图查看器的应用级入口:任意页面通过 useImageViewer().open(url) 打开,
// 只保留一个 Modal 与一套查看器状态。
import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react';
import { Modal } from 'react-native';

import { ZoomableImageViewer } from './ZoomableImageViewer';

type ImageViewerContextValue = {
  open: (url: string, alt?: string) => void;
};

const ImageViewerContext = createContext<ImageViewerContextValue | undefined>(undefined);

export function ImageViewerProvider({ children }: { children: ReactNode }) {
  const [image, setImage] = useState<{ url: string; alt?: string }>();

  const open = useCallback((url: string, alt?: string) => setImage({ url, alt }), []);
  const close = useCallback(() => setImage(undefined), []);
  const value = useMemo(() => ({ open }), [open]);

  return (
    <ImageViewerContext.Provider value={value}>
      {children}
      <Modal
        visible={image !== undefined}
        animationType="fade"
        supportedOrientations={['portrait']}
        onRequestClose={close}
      >
        {image ? (
          <ZoomableImageViewer
            key={image.url}
            imageUrl={image.url}
            alt={image.alt}
            onClose={close}
          />
        ) : null}
      </Modal>
    </ImageViewerContext.Provider>
  );
}

export function useImageViewer(): ImageViewerContextValue {
  const value = useContext(ImageViewerContext);
  if (!value) {
    throw new Error('useImageViewer must be used inside ImageViewerProvider');
  }
  return value;
}
