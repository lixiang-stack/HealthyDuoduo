import { makeMember } from '../test/fixtures';
import { childMembers, resolveCurrentChild } from './currentChild';

const doudou = makeMember({ member_id: 1, name: '豆豆', is_child: true });
const second = makeMember({ member_id: 2, name: '老二', is_child: true });
const mom = makeMember({ member_id: 3, name: '妈妈', is_child: false });

describe('current child resolution', () => {
  it('excludes adults from care targets', () => {
    expect(childMembers([doudou, mom])).toEqual([doudou]);
    expect(resolveCurrentChild([mom], doudou.member_id)).toEqual({ status: 'no-children' });
  });

  it('locks the only child even when the local selection is empty or stale', () => {
    expect(resolveCurrentChild([doudou, mom], undefined)).toEqual({
      status: 'selected',
      child: doudou,
      locked: true,
    });
    expect(resolveCurrentChild([doudou], second.member_id)).toEqual({
      status: 'selected',
      child: doudou,
      locked: true,
    });
  });

  it('restores the stored child when several children exist', () => {
    expect(resolveCurrentChild([doudou, second], second.member_id)).toEqual({
      status: 'selected',
      child: second,
      locked: false,
    });
  });

  it('asks for a selection when several children exist without a valid stored child', () => {
    expect(resolveCurrentChild([doudou, second], undefined)).toEqual({
      status: 'needs-selection',
    });
    expect(resolveCurrentChild([doudou, second], mom.member_id)).toEqual({
      status: 'needs-selection',
    });
  });
});
