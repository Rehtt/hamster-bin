import { useState } from 'react';

// 草稿保存整个业务表单及其关联输入；提交前必须合并当前尚未切换的表单。
export function usePhotoDrafts<T>() {
  const [drafts, setDrafts] = useState<T[]>([]);
  const [index, setIndex] = useState(0);
  const reset = () => { setDrafts([]); setIndex(0); };
  const start = (items: T[]) => { setDrafts(items); setIndex(0); };
  const merge = (current: T) => drafts.map((draft, i) => i === index ? current : draft);
  const select = (next: number, current: T) => {
    const updated = merge(current);
    setDrafts(updated);
    setIndex(next);
    return updated[next];
  };
  return { drafts, index, reset, start, merge, select };
}
