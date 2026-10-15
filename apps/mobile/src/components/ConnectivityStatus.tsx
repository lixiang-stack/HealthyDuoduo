// 连通状态的中性呈现(首页与设置页共用);文案停在事实侧,不做解读。
import { ActivityIndicator, StyleSheet, Text, View } from 'react-native';

import type { HealthState } from '../api/connectivity';
import { colors } from './theme';

export function ConnectivityStatus({ state }: { state: HealthState }) {
  if (state.status === 'checking') {
    return (
      <View style={styles.row}>
        <ActivityIndicator />
        <Text style={styles.detail}>正在检测…</Text>
      </View>
    );
  }
  if (state.status === 'unconfigured') {
    return <Text style={styles.detail}>尚未配置后端地址,请在设置中填写。</Text>;
  }
  if (state.status === 'ok') {
    return <Text style={styles.ok}>已连通 · 版本 {state.version}</Text>;
  }
  const title = state.status === 'unreachable' ? '无法连接后端' : '后端响应异常';
  return (
    <View>
      <Text style={styles.fail}>{title}</Text>
      <Text style={styles.detail}>{state.detail}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  ok: { fontSize: 16, color: colors.ok },
  fail: { fontSize: 16, color: colors.fail },
  detail: { fontSize: 13, color: colors.muted },
});
