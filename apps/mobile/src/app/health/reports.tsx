import { useLocalSearchParams } from 'expo-router';

import { ArchiveScreen } from '../../reports/ArchiveScreen';

// 健康档案 > 报告:按时间 / 按项目仍共用这一条二级路由,视图经 ?view= 保留。
export default function ReportsRoute() {
  const params = useLocalSearchParams<{ view?: string | string[] }>();
  const view = Array.isArray(params.view) ? params.view[0] : params.view;

  return <ArchiveScreen view={view === 'time' ? 'time' : 'panel'} />;
}
