// LineZoom 图像缩放至 OCR 行对应区域(核对三方对照的图像一侧;票 02 起详情页
// 段落卡共用):行框并集经 zoomCrop 定位(整段行文尽可能大地装进视口);原始尺寸
// 经 Image.getSize 取得。
import { useEffect, useState } from 'react';
import { Image, StyleSheet, Text, View } from 'react-native';

import type { OcrLineLink } from '../api/reports';
import { colors } from './theme';
import { unionBox, zoomCrop, type Size } from './zoom';

/** OCR 行缩放视口高(px):约三分之一屏宽,行文读到字级。 */
const ZOOM_VIEWPORT_HEIGHT = 96;

type ImageState = { url: string; natural?: Size; failed: boolean };

export function LineZoom({
  imageUrl,
  lines,
  testID,
  height = ZOOM_VIEWPORT_HEIGHT,
  highlight = false,
}: {
  imageUrl: string;
  lines: OcrLineLink[];
  testID?: string;
  /** 视口高度;原图定位用更大的视口,报告内条带保持默认 96。 */
  height?: number;
  /** 在原图上标出 OCR 行框;默认只裁切不描框。 */
  highlight?: boolean;
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
  const box = highlight ? unionBox(lines) : undefined;
  const scale = crop && current?.natural ? crop.width / current.natural.width : undefined;
  const highlightStyle =
    box && crop && scale
      ? {
          left: crop.left + box.x * scale,
          top: crop.top + box.y * scale,
          width: box.w * scale,
          height: box.h * scale,
        }
      : undefined;
  return (
    <View
      testID={testID}
      style={[styles.zoomViewport, { height }]}
      onLayout={(event) =>
        setViewport({ width: event.nativeEvent.layout.width, height })
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
      {highlightStyle ? (
        <View
          testID={testID ? `${testID}-highlight` : undefined}
          pointerEvents="none"
          style={[styles.highlight, highlightStyle]}
        />
      ) : null}
      {failed ? <Text style={styles.zoomHint}>图像区域载入失败,以上文原文与解析值为准</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  zoomViewport: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    marginTop: 4,
    overflow: 'hidden',
    justifyContent: 'center',
  },
  highlight: {
    position: 'absolute',
    borderWidth: 2,
    borderColor: colors.accent,
    borderRadius: 3,
  },
  zoomHint: { fontSize: 12, color: colors.muted, textAlign: 'center', paddingHorizontal: 8 },
});
