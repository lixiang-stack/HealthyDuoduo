// 成员列表排序口径单测(票 05):孩子优先,其余按创建序(同组也按创建序)。
import type { Member } from '../api/members';
import { sortMembers } from './order';

function member(memberId: number, name: string, isChild: boolean): Member {
  return {
    member_id: memberId,
    name,
    is_child: isChild,
    created_at: '2026-10-15T08:00:00Z',
  };
}

describe('sortMembers', () => {
  it('孩子排最前,不改变各组的创建序', () => {
    const input = [member(1, '奶奶', false), member(2, '小虎', true), member(3, '妹妹', true), member(4, '爸爸', false)];
    expect(sortMembers(input).map((m) => m.member_id)).toEqual([2, 3, 1, 4]);
  });

  it('后创建的孩子也排在先创建的成人前面', () => {
    const input = [member(1, '妈妈', false), member(2, '小虎', true)];
    expect(sortMembers(input).map((m) => m.name)).toEqual(['小虎', '妈妈']);
  });

  it('空列表返回空列表', () => {
    expect(sortMembers([])).toEqual([]);
  });

  it('created_at 同秒(服务端秒级精度)仍按 id 定序,不依赖时间戳', () => {
    const input = [member(2, '妹妹', true), member(1, '小虎', true)];
    expect(sortMembers(input).map((m) => m.member_id)).toEqual([1, 2]);
  });

  it('不改动调用方数组', () => {
    const input = [member(1, '妈妈', false), member(2, '小虎', true)];
    sortMembers(input);
    expect(input.map((m) => m.member_id)).toEqual([1, 2]);
  });
});
