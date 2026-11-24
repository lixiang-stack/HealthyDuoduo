import { DomainTabsLayout } from '../../navigation/DomainTabsLayout';

// B3 育儿域:底部只放今天 / 统计;记录是今天页动作,不是 tab。
export default function CareTabsLayout() {
  return <DomainTabsLayout domain="care" />;
}
