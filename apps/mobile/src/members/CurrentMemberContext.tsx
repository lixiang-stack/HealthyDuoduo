// App 级「当前成员 / 当前孩子」上下文:
// - 当前成员用于健康档案的查看对象(票 05);
// - 当前孩子用于育儿页的护理目标(票 03),只影响育儿,不影响健康档案。
// 两者都经 AsyncStorage 按设备持久化;删除当前成员 / 孩子时清除选择。
import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';

import {
  clearCurrentChildId,
  clearCurrentMemberId,
  loadCurrentChildId,
  loadCurrentMemberId,
  saveCurrentChildId,
  saveCurrentMemberId,
} from './currentMemberStore';

type CurrentMemberContextValue = {
  /** 当前成员 id;未选择或尚未从存储加载完为 undefined。 */
  currentMemberId: number | undefined;
  /** 切换当前成员;持久化失败不阻塞界面(最坏情况是重载后回到未选择)。 */
  selectMember: (memberId: number) => void;
  /** 清除当前成员(成员删除时);持久化失败不阻塞界面。 */
  clearMember: () => void;
  /** 当前孩子 id;未选择或尚未从存储加载完为 undefined。 */
  currentChildId: number | undefined;
  /** 本地选择是否已从存储加载完;育儿页据此避免把「加载中」当成「未选择」。 */
  childReady: boolean;
  /** 切换当前孩子;持久化失败不阻塞界面。 */
  setCurrentChild: (memberId: number) => void;
  /** 清除当前孩子(孩子删除 / 已不是孩子时);持久化失败不阻塞界面。 */
  clearChild: () => void;
};

const CurrentMemberContext = createContext<CurrentMemberContextValue | undefined>(undefined);

export function CurrentMemberProvider({ children }: { children: ReactNode }) {
  const [currentMemberId, setCurrentMemberId] = useState<number>();
  const [currentChildId, setCurrentChildId] = useState<number>();
  const [childReady, setChildReady] = useState(false);

  useEffect(() => {
    let cancelled = false;
    // 任一存储读取失败都不应让育儿页永久停在「加载中」;能读到的选择照常恢复,
    // 读不到的选择回到未选择,后续由成员列表回落。
    void Promise.allSettled([loadCurrentMemberId(), loadCurrentChildId()]).then(
      ([memberId, childId]) => {
        if (cancelled) {
          return;
        }
        if (memberId.status === 'fulfilled') {
          setCurrentMemberId(memberId.value);
        }
        if (childId.status === 'fulfilled') {
          setCurrentChildId(childId.value);
        }
        setChildReady(true);
      },
    );
    return () => {
      cancelled = true;
    };
  }, []);

  const selectMember = useCallback((memberId: number) => {
    setCurrentMemberId(memberId);
    void saveCurrentMemberId(memberId);
  }, []);

  const clearMember = useCallback(() => {
    setCurrentMemberId(undefined);
    void clearCurrentMemberId();
  }, []);

  const setCurrentChild = useCallback((memberId: number) => {
    setCurrentChildId(memberId);
    void saveCurrentChildId(memberId);
  }, []);

  const clearChild = useCallback(() => {
    setCurrentChildId(undefined);
    void clearCurrentChildId();
  }, []);

  return (
    <CurrentMemberContext.Provider
      value={{
        currentMemberId,
        selectMember,
        clearMember,
        currentChildId,
        childReady,
        setCurrentChild,
        clearChild,
      }}
    >
      {children}
    </CurrentMemberContext.Provider>
  );
}

export function useCurrentMember(): CurrentMemberContextValue {
  const value = useContext(CurrentMemberContext);
  if (!value) {
    throw new Error('useCurrentMember must be used inside CurrentMemberProvider');
  }
  return value;
}
