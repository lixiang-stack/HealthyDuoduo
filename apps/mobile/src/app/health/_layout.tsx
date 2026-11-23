import { DomainTabsLayout } from '../../navigation/DomainTabsLayout';

// B3 健康档案域:底部只放报告 / 趋势;报告详情 / 核对 / 趋势详情仍走根 Stack。
export default function HealthTabsLayout() {
  return <DomainTabsLayout domain="health" />;
}
