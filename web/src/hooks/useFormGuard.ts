import { useEffect, useRef, useState } from 'react';
import { useConfirm } from '../context/confirm';
// 使用显式初始快照，不把恢复任务结果当成已保存内容。
export function useFormGuard(value: unknown) {
  const [baseline, setBaseline] = useState('');
  const fingerprint = JSON.stringify(value);
  const confirm = useConfirm();
  const dirty = baseline !== fingerprint;
  const reset = (initial: unknown) => setBaseline(JSON.stringify(initial));
  const allowClose = async () => !dirty || await confirm({ title: '放弃未保存的修改？', message: '关闭后，本次尚未保存的表单内容将丢失。后台解析任务仍可在任务中心查看。', confirmLabel: '放弃修改' });
  return { dirty, reset, allowClose };
}
export function useSubmitLock() {
  const lock = useRef(false);
  const [busy, setBusy] = useState(false);
  useEffect(() => () => { lock.current = false; }, []);
  return { busy, run: async (action: () => Promise<void>) => { if (lock.current) return; lock.current = true; setBusy(true); try { await action(); } finally { lock.current = false; setBusy(false); } } };
}
