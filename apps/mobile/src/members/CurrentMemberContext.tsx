// App 级「当前成员」上下文(票 05 备好;票 06+ 消费:时间线过滤、上传归属默认)。
// 选择经 AsyncStorage 持久化;删除当前成员时清除选择,下次启动回到未选择态。
import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';

import { clearCurrentMemberId, loadCurrentMemberId, saveCurrentMemberId } from './currentMemberStore';

type CurrentMemberContextValue = {
  /** 当前成员 id;未选择或尚未从存储加载完为 undefined。 */
  currentMemberId: number | undefined;
  /** 切换当前成员;持久化失败不阻塞界面(最坏情况是重载后回到未选择)。 */
  selectMember: (memberId: number) => void;
  /** 清除当前成员(成员删除时);持久化失败不阻塞界面。 */
  clearMember: () => void;
};

const CurrentMemberContext = createContext<CurrentMemberContextValue | undefined>(undefined);

export function CurrentMemberProvider({ children }: { children: ReactNode }) {
  const [currentMemberId, setCurrentMemberId] = useState<number>();

  useEffect(() => {
    void loadCurrentMemberId().then(setCurrentMemberId);
  }, []);

  const selectMember = useCallback((memberId: number) => {
    setCurrentMemberId(memberId);
    void saveCurrentMemberId(memberId);
  }, []);

  const clearMember = useCallback(() => {
    setCurrentMemberId(undefined);
    void clearCurrentMemberId();
  }, []);

  return (
    <CurrentMemberContext.Provider value={{ currentMemberId, selectMember, clearMember }}>
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
