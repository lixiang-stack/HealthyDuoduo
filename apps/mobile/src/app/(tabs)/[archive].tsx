import { Redirect, useLocalSearchParams } from 'expo-router';

import { ArchiveScreen } from '../../reports/ArchiveScreen';

/** 档案页的两个真实 URL 共用同一 Tab 路由,避免隐藏 Tab 路由造成双份挂载。 */
export default function ArchiveRoute() {
  const { archive } = useLocalSearchParams<{ archive?: string | string[] }>();
  const view = Array.isArray(archive) ? archive[0] : archive;

  if (view === 'panels') {
    return <ArchiveScreen view="panel" />;
  }
  if (view === 'timeline') {
    return <ArchiveScreen view="time" />;
  }
  return <Redirect href="/panels" />;
}
