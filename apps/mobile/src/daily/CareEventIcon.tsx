import Svg, { Path } from 'react-native-svg';

import type { CareEventType } from '../care/careEvents';

/** Small type icons used inside the timeline rail nodes. */
export function CareEventIcon({
  type,
  color,
  size,
}: {
  type: CareEventType;
  color: string;
  size: number;
}) {
  switch (type) {
    case 'feed':
      return (
        <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
          <Path
            d="M9 3h6v3l2 2v13H7V8l2-2V3Z"
            stroke={color}
            strokeWidth={1.8}
            strokeLinejoin="round"
          />
          <Path d="M9 11h6" stroke={color} strokeWidth={1.8} strokeLinecap="round" />
        </Svg>
      );
    case 'diaper':
      return (
        <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
          <Path
            d="M12 3s6 6.5 6 10a6 6 0 1 1-12 0c0-3.5 6-10 6-10Z"
            stroke={color}
            strokeWidth={1.8}
            strokeLinejoin="round"
          />
        </Svg>
      );
    case 'sleep':
      return (
        <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
          <Path
            d="M20 14.5A8.5 8.5 0 0 1 9.5 4 8.5 8.5 0 1 0 20 14.5Z"
            stroke={color}
            strokeWidth={1.8}
            strokeLinejoin="round"
          />
        </Svg>
      );
    case 'solid':
      return (
        <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
          <Path
            d="M4 11h16a8 8 0 0 1-16 0Z"
            stroke={color}
            strokeWidth={1.8}
            strokeLinejoin="round"
          />
          <Path
            d="M8 3v4M12 2v5M16 3v4"
            stroke={color}
            strokeWidth={1.8}
            strokeLinecap="round"
          />
        </Svg>
      );
  }
}
