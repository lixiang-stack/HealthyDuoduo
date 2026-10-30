import {
  clampScale,
  clampTransform,
  DOUBLE_TAP_SCALE,
  fitSize,
  MAX_SCALE,
  MIN_SCALE,
  toggleDoubleTap,
  zoomAround,
} from './geometry';

describe('fitSize', () => {
  it('按 contain 适配:宽高都不超过视口，保持等比', () => {
    expect(fitSize({ width: 1000, height: 500 }, { width: 300, height: 300 })).toEqual({
      width: 300,
      height: 150,
    });
    expect(fitSize({ width: 500, height: 1000 }, { width: 300, height: 300 })).toEqual({
      width: 150,
      height: 300,
    });
  });

  it('退化输入返回 undefined', () => {
    expect(fitSize({ width: 0, height: 100 }, { width: 300, height: 300 })).toBeUndefined();
    expect(fitSize({ width: 100, height: 100 }, { width: 0, height: 300 })).toBeUndefined();
  });
});

describe('clampScale', () => {
  it('夹紧到 1×–4×', () => {
    expect(clampScale(0.5)).toBe(MIN_SCALE);
    expect(clampScale(1)).toBe(1);
    expect(clampScale(2.5)).toBe(2.5);
    expect(clampScale(9)).toBe(MAX_SCALE);
  });
});

describe('clampTransform', () => {
  const fit = { width: 300, height: 200 };
  const viewport = { width: 400, height: 800 };

  it('1× 时无论平移多少都居中', () => {
    expect(
      clampTransform({ scale: 1, translateX: 999, translateY: -999 }, fit, viewport),
    ).toEqual({ scale: 1, translateX: 0, translateY: 0 });
  });

  it('放大后按可移动区间夹紧:宽向可移，高向仍小于视口', () => {
    expect(
      clampTransform({ scale: 2, translateX: 999, translateY: 999 }, fit, viewport),
    ).toEqual({ scale: 2, translateX: 100, translateY: 0 });
    expect(
      clampTransform({ scale: 2, translateX: -999, translateY: -999 }, fit, viewport),
    ).toEqual({ scale: 2, translateX: -100, translateY: 0 });
  });
});

describe('zoomAround', () => {
  const fit = { width: 300, height: 600 };
  const viewport = { width: 300, height: 600 };
  const center = { x: 150, y: 300 };

  it('焦点下的图像点在缩放前后保持不动', () => {
    const focal = { x: 120, y: 200 };
    const start = { scale: 1, translateX: 0, translateY: 0 };
    const next = zoomAround(start, 2, focal, fit, viewport);
    const point = { x: focal.x - center.x, y: focal.y - center.y };
    expect(next.scale).toBe(2);
    expect(center.x + next.translateX + point.x * next.scale).toBeCloseTo(focal.x, 5);
    expect(center.y + next.translateY + point.y * next.scale).toBeCloseTo(focal.y, 5);
  });

  it('焦点缩放后仍按边界夹紧', () => {
    const next = zoomAround(
      { scale: 1, translateX: 0, translateY: 0 },
      MAX_SCALE,
      { x: 0, y: 0 },
      fit,
      viewport,
    );
    expect(next.translateX).toBeGreaterThanOrEqual(-(fit.width * MAX_SCALE - viewport.width) / 2);
    expect(next.translateX).toBeLessThanOrEqual((fit.width * MAX_SCALE - viewport.width) / 2);
    expect(next.translateY).toBeGreaterThanOrEqual(-(fit.height * MAX_SCALE - viewport.height) / 2);
    expect(next.translateY).toBeLessThanOrEqual((fit.height * MAX_SCALE - viewport.height) / 2);
  });
});

describe('toggleDoubleTap', () => {
  const fit = { width: 300, height: 600 };
  const viewport = { width: 300, height: 600 };

  it('1× 双击放大到 2.5×', () => {
    const next = toggleDoubleTap(
      { scale: 1, translateX: 0, translateY: 0 },
      { x: 150, y: 300 },
      fit,
      viewport,
    );
    expect(next.scale).toBe(DOUBLE_TAP_SCALE);
  });

  it('已放大时双击回到 1× 居中', () => {
    expect(
      toggleDoubleTap({ scale: 2.5, translateX: 80, translateY: 40 }, { x: 0, y: 0 }, fit, viewport),
    ).toEqual({ scale: 1, translateX: 0, translateY: 0 });
  });
});
