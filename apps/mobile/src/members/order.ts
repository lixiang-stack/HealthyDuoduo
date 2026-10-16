// 成员列表展示口径(纯逻辑):孩子优先,其余按创建序。
// member_id 为服务端 BIGSERIAL 自增主键(不重用、不删除),id 升序即创建序;
// created_at 为秒级精度且用于展示,不作排序依据(同秒创建不歧义)。
import type { Member } from '../api/members';

export function sortMembers(members: Member[]): Member[] {
  return [...members].sort(
    (a, b) => Number(b.is_child) - Number(a.is_child) || a.member_id - b.member_id,
  );
}
