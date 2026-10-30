// 原图查看器的纯几何:contain 适配、缩放夹紧、焦点缩放与双击目标。
// 手势回调在 UI 线程执行,涉及的函数标 worklet;纯函数可在 Jest 直测。
export type Size = { width: number; height: number };
export type Point = { x: number; y: number };
export type Transform = { scale: number; translateX: number; translateY: number };

export const MIN_SCALE = 1;
export const MAX_SCALE = 4;
export const DOUBLE_TAP_SCALE = 2.5;

/** contain 适配后的渲染尺寸;退化输入返回 undefined。 */
export function fitSize(natural: Size, viewport: Size): Size | undefined {
  if (
    natural.width <= 0 ||
    natural.height <= 0 ||
    viewport.width <= 0 ||
    viewport.height <= 0
  ) {
    return undefined;
  }
  const scale = Math.min(viewport.width / natural.width, viewport.height / natural.height);
  return { width: natural.width * scale, height: natural.height * scale };
}

/** 缩放范围夹紧到 [1, 4]。 */
export function clampScale(scale: number): number {
  'worklet';
  return Math.min(MAX_SCALE, Math.max(MIN_SCALE, scale));
}

/** 单轴平移夹紧:rendered <= viewport 时居中(0),否则允许两侧露出不超过一半。 */
function clampAxis(translate: number, rendered: number, viewport: number): number {
  'worklet';
  if (rendered <= viewport) {
    return 0;
  }
  const max = (rendered - viewport) / 2;
  return Math.min(max, Math.max(-max, translate));
}

/** 平移夹紧:图像小于视口的方向居中;大于视口的方向不露图外。 */
export function clampTransform(transform: Transform, fit: Size, viewport: Size): Transform {
  'worklet';
  const scale = clampScale(transform.scale);
  return {
    scale,
    translateX: clampAxis(transform.translateX, fit.width * scale, viewport.width),
    translateY: clampAxis(transform.translateY, fit.height * scale, viewport.height),
  };
}

/** 以 focal(视口坐标)为不动点缩放到 nextScale,再夹紧平移。 */
export function zoomAround(
  transform: Transform,
  nextScale: number,
  focal: Point,
  fit: Size,
  viewport: Size,
): Transform {
  'worklet';
  const scale = clampScale(nextScale);
  const ratio = scale / transform.scale;
  const centerX = viewport.width / 2;
  const centerY = viewport.height / 2;
  return clampTransform(
    {
      scale,
      translateX: focal.x - centerX - (focal.x - centerX - transform.translateX) * ratio,
      translateY: focal.y - centerY - (focal.y - centerY - transform.translateY) * ratio,
    },
    fit,
    viewport,
  );
}

/** 双击:已放大则回到 1×,否则以 focal 为不动点放大到 2.5×。 */
export function toggleDoubleTap(
  transform: Transform,
  focal: Point,
  fit: Size,
  viewport: Size,
): Transform {
  'worklet';
  if (transform.scale > MIN_SCALE) {
    return { scale: MIN_SCALE, translateX: 0, translateY: 0 };
  }
  return zoomAround(transform, DOUBLE_TAP_SCALE, focal, fit, viewport);
}
