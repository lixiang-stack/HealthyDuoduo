// 跨页面/组件复用的基础样式原语(与 theme 色板同层):反复出现的横向行与说明文字
// 集中一处,避免第 N 份拷贝漂移;页面/组件仍各自持有布局式样。
import { StyleSheet } from 'react-native';

import { colors } from './theme';

export const sharedStyles = StyleSheet.create({
  horizontal: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  label: { fontSize: 13, color: colors.muted },
  muted: { fontSize: 13, color: colors.muted, lineHeight: 20 },
});
