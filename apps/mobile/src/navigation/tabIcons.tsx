import type { ReactElement } from 'react';
import type { ColorValue } from 'react-native';
import Svg, { Line, Path, Rect } from 'react-native-svg';

export type DomainTabIcon = (props: { color: ColorValue; size: number }) => ReactElement;

export function TodayIcon({ color, size }: { color: ColorValue; size: number }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      <Path
        d="M4 10.5 12 4l8 6.5V20a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-9.5Z"
        stroke={color}
        strokeWidth={1.7}
        strokeLinejoin="round"
      />
    </Svg>
  );
}

export function StatsIcon({ color, size }: { color: ColorValue; size: number }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      <Rect x={4} y={12} width={4} height={8} rx={1} stroke={color} strokeWidth={1.7} />
      <Rect x={10} y={7} width={4} height={13} rx={1} stroke={color} strokeWidth={1.7} />
      <Rect x={16} y={4} width={4} height={16} rx={1} stroke={color} strokeWidth={1.7} />
    </Svg>
  );
}

export function ReportsIcon({ color, size }: { color: ColorValue; size: number }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      <Line x1={4} y1={7} x2={20} y2={7} stroke={color} strokeWidth={1.7} strokeLinecap="round" />
      <Line x1={4} y1={12} x2={20} y2={12} stroke={color} strokeWidth={1.7} strokeLinecap="round" />
      <Line x1={4} y1={17} x2={20} y2={17} stroke={color} strokeWidth={1.7} strokeLinecap="round" />
    </Svg>
  );
}

export function TrendsIcon({ color, size }: { color: ColorValue; size: number }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      <Path
        d="m4 16 5-5 4 3 6-7"
        stroke={color}
        strokeWidth={1.7}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </Svg>
  );
}
