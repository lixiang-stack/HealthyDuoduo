import { DomainTabsLayout } from '../../navigation/DomainTabsLayout';

// B3 日常域:底部只放今天 / 统计;记录是今天页动作,不是 tab。
export default function DailyTabsLayout() {
  return <DomainTabsLayout domain="daily" />;
}
