// 全屏原图查看器:双指 1×–4×、放大后单指拖动、双击 1×/2.5×、重置。
// 只负责一张图的查看与手势;打开 / 关闭由 ImageViewerProvider 管。
import { StatusBar } from 'expo-status-bar';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Image, Pressable, StyleSheet, Text, View } from 'react-native';
import { Gesture, GestureDetector, GestureHandlerRootView } from 'react-native-gesture-handler';
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from 'react-native-reanimated';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { colors } from '../theme';
import {
  clampTransform,
  fitSize,
  toggleDoubleTap,
  zoomAround,
  type Size,
  type Transform,
} from './geometry';

const TIMING = { duration: 180 };
const OVERLAY_BACKGROUND = 'rgba(255,255,255,0.14)';

export function ZoomableImageViewer({
  imageUrl,
  alt,
  onClose,
}: {
  imageUrl: string;
  alt?: string;
  onClose: () => void;
}) {
  const insets = useSafeAreaInsets();
  const [viewport, setViewport] = useState<Size>();
  const [loaded, setLoaded] = useState<{ url: string; natural?: Size; failed: boolean }>();

  useEffect(() => {
    let cancelled = false;
    Image.getSize(
      imageUrl,
      (width, height) => {
        if (!cancelled) {
          setLoaded({ url: imageUrl, natural: { width, height }, failed: false });
        }
      },
      () => {
        if (!cancelled) {
          setLoaded({ url: imageUrl, failed: true });
        }
      },
    );
    return () => {
      cancelled = true;
    };
  }, [imageUrl]);

  // 只认当前 URL 的加载结果;URL 变化时旧结果自然失效,无需在 effect 里同步清状态。
  const current = loaded?.url === imageUrl ? loaded : undefined;
  const natural = current?.natural;
  const failed = current?.failed ?? false;
  const fit = viewport && natural ? fitSize(natural, viewport) : undefined;

  return (
    <GestureHandlerRootView style={styles.root}>
      <StatusBar style="light" />
      <View
        testID="image-viewer-root"
        style={styles.root}
        onLayout={(event) => {
          const { width, height } = event.nativeEvent.layout;
          setViewport({ width, height });
        }}
      >
        {fit && viewport ? (
          <ZoomableCanvas imageUrl={imageUrl} alt={alt} fit={fit} viewport={viewport} />
        ) : failed ? (
          <Text style={styles.fail}>图像载入失败</Text>
        ) : (
          <ActivityIndicator color={colors.onAccent} />
        )}
        <Pressable
          testID="image-viewer-close"
          accessibilityRole="button"
          accessibilityLabel="关闭原图"
          onPress={onClose}
          style={[styles.close, { top: insets.top + 8, right: insets.right + 16 }]}
        >
          <Text style={styles.closeText}>关闭</Text>
        </Pressable>
        <Text style={[styles.hint, { bottom: insets.bottom + 16 }]}>
          双指缩放 · 双击放大 · 拖动查看
        </Text>
      </View>
    </GestureHandlerRootView>
  );
}

function ZoomableCanvas({
  imageUrl,
  alt,
  fit,
  viewport,
}: {
  imageUrl: string;
  alt?: string;
  fit: Size;
  viewport: Size;
}) {
  const scale = useSharedValue(1);
  const translateX = useSharedValue(0);
  const translateY = useSharedValue(0);
  const startScale = useSharedValue(1);
  const startX = useSharedValue(0);
  const startY = useSharedValue(0);

  const reset = useCallback(() => {
    scale.set(withTiming(1, TIMING));
    translateX.set(withTiming(0, TIMING));
    translateY.set(withTiming(0, TIMING));
  }, [scale, translateX, translateY]);

  const pinch = Gesture.Pinch()
    .onStart(() => {
      startScale.set(scale.get());
      startX.set(translateX.get());
      startY.set(translateY.get());
    })
    .onUpdate((event) => {
      const start: Transform = {
        scale: startScale.get(),
        translateX: startX.get(),
        translateY: startY.get(),
      };
      const next = zoomAround(
        start,
        startScale.get() * event.scale,
        { x: event.focalX, y: event.focalY },
        fit,
        viewport,
      );
      scale.set(next.scale);
      translateX.set(next.translateX);
      translateY.set(next.translateY);
    });

  const pan = Gesture.Pan()
    .maxPointers(1)
    .onStart(() => {
      startX.set(translateX.get());
      startY.set(translateY.get());
    })
    .onUpdate((event) => {
      const next = clampTransform(
        {
          scale: scale.get(),
          translateX: startX.get() + event.translationX,
          translateY: startY.get() + event.translationY,
        },
        fit,
        viewport,
      );
      translateX.set(next.translateX);
      translateY.set(next.translateY);
    });

  const doubleTap = Gesture.Tap()
    .numberOfTaps(2)
    .onEnd((event) => {
      const next = toggleDoubleTap(
        { scale: scale.get(), translateX: translateX.get(), translateY: translateY.get() },
        { x: event.x, y: event.y },
        fit,
        viewport,
      );
      scale.set(withTiming(next.scale, TIMING));
      translateX.set(withTiming(next.translateX, TIMING));
      translateY.set(withTiming(next.translateY, TIMING));
    });

  const gesture = Gesture.Simultaneous(pinch, pan, doubleTap);
  const frameStyle = useAnimatedStyle(() => ({
    transform: [{ translateX: translateX.value }, { translateY: translateY.value }],
  }));
  const imageStyle = useAnimatedStyle(() => ({
    transform: [{ scale: scale.value }],
  }));

  return (
    <GestureDetector gesture={gesture}>
      <View style={styles.canvas}>
        <Animated.View
          style={[
            styles.frame,
            {
              width: fit.width,
              height: fit.height,
              left: (viewport.width - fit.width) / 2,
              top: (viewport.height - fit.height) / 2,
            },
            frameStyle,
          ]}
        >
          <Animated.Image
            testID="image-viewer-image"
            source={{ uri: imageUrl }}
            accessibilityLabel={alt ?? '检查单原图'}
            style={[{ width: fit.width, height: fit.height }, imageStyle]}
            resizeMode="contain"
          />
        </Animated.View>
        <Pressable
          testID="image-viewer-reset"
          accessibilityRole="button"
          accessibilityLabel="重置缩放"
          onPress={reset}
          style={styles.reset}
        >
          <Text style={styles.resetText}>重置</Text>
        </Pressable>
      </View>
    </GestureDetector>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: colors.text, justifyContent: 'center', alignItems: 'center' },
  canvas: { flex: 1, width: '100%', height: '100%', overflow: 'hidden' },
  frame: { position: 'absolute' },
  close: {
    position: 'absolute',
    paddingVertical: 8,
    paddingHorizontal: 12,
    borderRadius: 16,
    backgroundColor: OVERLAY_BACKGROUND,
  },
  closeText: { color: colors.onAccent, fontSize: 15 },
  hint: {
    position: 'absolute',
    left: 0,
    right: 0,
    textAlign: 'center',
    color: colors.onAccent,
    fontSize: 12,
    opacity: 0.72,
  },
  reset: {
    position: 'absolute',
    bottom: 56,
    left: '50%',
    marginLeft: -34,
    width: 68,
    alignItems: 'center',
    paddingVertical: 8,
    borderRadius: 16,
    backgroundColor: OVERLAY_BACKGROUND,
  },
  resetText: { color: colors.onAccent, fontSize: 14 },
  fail: { color: colors.onAccent, fontSize: 14 },
});
