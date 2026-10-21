// LineZoom 图像缩放至 OCR 行对应区域(核对三方对照的图像一侧;票 02 起详情页
// 段落卡共用):行框并集经 zoomCrop 定位(整段行文尽可能大地装进视口);原始尺寸
// 经 Image.getSize 取得。
import { useEffect, useState } from 'react';
import { Image, StyleSheet, Text, View } from 'react-native';

import type { OcrLineLink } from '../api/reports';
import { colors } from './theme';
import { zoomCrop, type Size } from './zoom';

/** OCR 行缩放视口高(px):约三分之一屏宽,行文读到字级。 */
const ZOOM_VIEWPORT_HEIGHT = 96;

type ImageState = { url: string; natural?: Size; failed: boolean };

export function LineZoom({
  imageUrl,
  lines,
  testID,
}: {
  imageUrl: string;
  lines: OcrLineLink[];
  testID?: string;
}) {
  const [viewport, setViewport] = useState<Size>();
  const [imageState, setImageState] = useState<ImageState>();

  useEffect(() => {
    let cancelled = false;
    Image.getSize(
      imageUrl,
      (width, height) => {
        if (!cancelled) {
          setImageState({ url: imageUrl, natural: { width, height }, failed: false });
        }
      },
      () => {
        if (!cancelled) {
          setImageState({ url: imageUrl, failed: true });
        }
      },
    );
    return () => {
      cancelled = true;
    };
  }, [imageUrl]);

  // 只认当前 URL 的加载结果;URL 变化时旧结果自然失效,无需在 effect 里同步清状态。
  const current = imageState?.url === imageUrl ? imageState : undefined;
  const crop = viewport && current?.natural ? zoomCrop(current.natural, lines, viewport) : undefined;
  const failed = current?.failed ?? false;
  return (
    <View
      testID={testID}
      style={styles.zoomViewport}
      onLayout={(event) =>
        setViewport({ width: event.nativeEvent.layout.width, height: ZOOM_VIEWPORT_HEIGHT })
      }
    >
      {crop ? (
        <Image
          source={{ uri: imageUrl }}
          style={{
            position: 'absolute',
            left: crop.left,
            top: crop.top,
            width: crop.width,
            height: crop.height,
          }}
        />
      ) : null}
      {failed ? <Text style={styles.zoomHint}>图像区域载入失败,以上文原文与解析值为准</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  zoomViewport: {
    height: ZOOM_VIEWPORT_HEIGHT,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    marginTop: 4,
    overflow: 'hidden',
    justifyContent: 'center',
  },
  zoomHint: { fontSize: 12, color: colors.muted, textAlign: 'center', paddingHorizontal: 8 },
});
