// 图像缩放定位(票 09 三方对照的「图像缩放至 OCR 行对应区域」):行框 → 视口内
// 图像摆放几何。口径:取行框并集,外扩一成上下文;整段行文尽可能大地装进视口
// (等比,留一成边),再平移把区域中心对准视口中心;图像小于视口的方向居中,
// 大于视口的方向夹紧边缘(不露出图外)。纯几何,可直测。
import type { OcrLineLink } from '../api/reports';

export type Size = { width: number; height: number };
export type Rect = { left: number; top: number; width: number; height: number };

/** 缩放留边:区域占视口的最大比例(留出视觉边距)。 */
const FIT = 0.9;
/** 上下文外扩:以行框最大边的一成作上下左右边距,让行文有周边参照。 */
const CONTEXT = 0.1;

/** 行框并集(四点多边形无关顶点序,取轴对齐包围盒);无行链接返回 undefined。 */
export function unionBox(lines: OcrLineLink[]): { x: number; y: number; w: number; h: number } | undefined {
  if (lines.length === 0) {
    return undefined;
  }
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const link of lines) {
    for (const [x, y] of link.box) {
      minX = Math.min(minX, x);
      minY = Math.min(minY, y);
      maxX = Math.max(maxX, x);
      maxY = Math.max(maxY, y);
    }
  }
  return { x: minX, y: minY, w: maxX - minX, h: maxY - minY };
}

/** 图像在视口内的摆放几何(位置与缩放后的整图尺寸);无行链接或尺寸退化返回 undefined。 */
export function zoomCrop(natural: Size, lines: OcrLineLink[], viewport: Size): Rect | undefined {
  const box = unionBox(lines);
  if (!box || natural.width <= 0 || natural.height <= 0 || viewport.width <= 0 || viewport.height <= 0) {
    return undefined;
  }
  const maxSide = Math.max(box.w, box.h);
  if (maxSide <= 0) {
    return undefined;
  }
  const margin = CONTEXT * maxSide;
  const regionW = box.w + 2 * margin;
  const regionH = box.h + 2 * margin;
  const scale = Math.min((FIT * viewport.width) / regionW, (FIT * viewport.height) / regionH);
  const width = natural.width * scale;
  const height = natural.height * scale;
  const regionCx = box.x + box.w / 2;
  const regionCy = box.y + box.h / 2;
  return {
    left: clampOffset(viewport.width / 2 - regionCx * scale, width, viewport.width),
    top: clampOffset(viewport.height / 2 - regionCy * scale, height, viewport.height),
    width,
    height,
  };
}

/** 平移夹紧:图像小于视口 → 居中;大于视口 → 不露图外(偏移落在可滚动区间)。 */
function clampOffset(offset: number, rendered: number, viewport: number): number {
  if (rendered <= viewport) {
    return (viewport - rendered) / 2;
  }
  return Math.min(0, Math.max(viewport - rendered, offset));
}
