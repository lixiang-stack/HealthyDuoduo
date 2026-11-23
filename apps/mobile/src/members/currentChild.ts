import type { Member } from '../api/members';

/** 护理目标只可能是孩子;成人成员不进入选择器。 */
export function childMembers(members: Member[]): Member[] {
  return members.filter((member) => member.is_child);
}

/**
 * 每台设备的当前孩子:
 * - 没有孩子 → 育儿页空态,默认入口回落健康档案;
 * - 只有一个孩子 → 自动选中并锁定,不需要选择器;
 * - 多个孩子 → 恢复本地选择;没有有效选择时要求选择。
 */
export type CurrentChildState =
  | { status: 'no-children' }
  | { status: 'needs-selection' }
  | { status: 'selected'; child: Member; locked: boolean };

export function resolveCurrentChild(
  members: Member[],
  currentChildId: number | undefined,
): CurrentChildState {
  const children = childMembers(members);
  if (children.length === 0) {
    return { status: 'no-children' };
  }
  if (children.length === 1) {
    return { status: 'selected', child: children[0], locked: true };
  }
  const selected = children.find((child) => child.member_id === currentChildId);
  return selected
    ? { status: 'selected', child: selected, locked: false }
    : { status: 'needs-selection' };
}
