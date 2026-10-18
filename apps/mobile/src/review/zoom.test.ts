import type { OcrLineLink } from '../api/reports';

import { unionBox, zoomCrop } from './zoom';

// 行链接形状与共享栈真实报告同源(识别端坐标 = 图像像素;行框 40×20 的表格行)。
function line(x: number, y: number, w = 40, h = 20): OcrLineLink {
  return {
    line: 0,
    box: [
      [x, y],
      [x + w, y],
      [x + w, y + h],
      [x, y + h],
    ],
  };
}

describe('unionBox', () => {
  it('多点任意顶点序取轴对齐并集;空行链接返回 undefined', () => {
    expect(unionBox([])).toBeUndefined();
    const box = unionBox([line(100, 50), line(300, 50), line(200, 90)]);
    expect(box).toEqual({ x: 100, y: 50, w: 240, h: 60 });
  });
});

describe('zoomCrop', () => {
  const natural = { width: 1000, height: 1500 };

  it('行区域尽可能大地装进视口:等比、留边、区域中心对准视口中心', () => {
    // 区域 = 行框并集(40×20)+ 一成最大边上下文(4) → 48×28;
    // scale = min(0.9×300/48, 0.9×100/28) = min(5.625, 3.214) = 3.214…(高度受限)。
    const viewport = { width: 300, height: 100 };
    const crop = zoomCrop(natural, [line(100, 200)], viewport)!;
    expect(crop.width).toBeCloseTo(1000 * (90 / 28), 5);
    expect(crop.height).toBeCloseTo(1500 * (90 / 28), 5);
    // 区域中心 (120, 210) 落在视口中心。
    const centerX = crop.left + 120 * (90 / 28);
    const centerY = crop.top + 210 * (90 / 28);
    expect(centerX).toBeCloseTo(150, 5);
    expect(centerY).toBeCloseTo(50, 5);
  });

  it('图像大于视口的方向夹紧:不露出图外(偏移落在 [viewport-rendered, 0])', () => {
    const viewport = { width: 300, height: 100 };
    // 区域贴图像左上角:不做夹紧会露白,夹紧后左上角对齐视口左上。
    const corner = zoomCrop(natural, [line(0, 0)], viewport)!;
    expect(corner.left).toBe(0);
    expect(corner.top).toBe(0);
    // 区域贴图像右下角:夹到右缘/下缘(图像右/下边与视口右/下边对齐)。
    const bottomRight = zoomCrop(natural, [line(960, 1480)], viewport)!;
    expect(bottomRight.left).toBeCloseTo(viewport.width - bottomRight.width, 5);
    expect(bottomRight.top).toBeCloseTo(viewport.height - bottomRight.height, 5);
  });

  it('图像小于视口的方向居中', () => {
    // 小图 + 大视口:渲染尺寸两个方向都小于视口 → 居中(等边距)。
    const crop = zoomCrop({ width: 60, height: 30 }, [line(0, 0, 40, 20)], { width: 300, height: 100 })!;
    expect(crop.width).toBeLessThan(300);
    expect(crop.height).toBeLessThan(100);
    expect(crop.left).toBeCloseTo((300 - crop.width) / 2, 5);
    expect(crop.top).toBeCloseTo((100 - crop.height) / 2, 5);
  });

  it('退化输入返回 undefined:无行链接、图像/视口零尺寸、零面积行框', () => {
    expect(zoomCrop(natural, [], { width: 300, height: 100 })).toBeUndefined();
    expect(zoomCrop({ width: 0, height: 0 }, [line(1, 1)], { width: 300, height: 100 })).toBeUndefined();
    expect(zoomCrop(natural, [line(1, 1)], { width: 0, height: 0 })).toBeUndefined();
    expect(zoomCrop(natural, [line(5, 5, 0, 0)], { width: 300, height: 100 })).toBeUndefined();
  });
});
