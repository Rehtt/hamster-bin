import { useEffect, useRef, useState } from 'react';
import { toast } from 'react-hot-toast';
import { taskPending, useTasks } from '../context/tasks';
import type { TaskRequest, TaskResult } from '../types';

// 每次打开/关闭表单使会话失效；只在当前会话消费一次结果。
export function useParseTask<T>(onResult: (result: TaskResult, snapshot: T) => void) {
  const { tasks, submit } = useTasks();
  const session = useRef(0);
  const submitting = useRef(false);
  const consumed = useRef<number | null>(null);
  const [pending, setPending] = useState<{ id: number; snapshot: T } | null>(null);
  const [sending, setSending] = useState(false);
  const task = pending ? tasks[pending.id] : undefined;
  const busy = sending || taskPending(task);
  const reset = () => { session.current++; submitting.current = false; setPending(null); setSending(false); };
  useEffect(() => {
    if (!pending || !task || taskPending(task) || consumed.current === task.id) return;
    consumed.current = task.id;
    if (task.status === 'succeeded' && task.result) onResult(task.result, pending.snapshot);
  }, [pending, task, onResult]);
  const start = async (request: TaskRequest, snapshot: T) => {
    if (submitting.current || busy) return;
    const current = session.current;
    submitting.current = true;
    setSending(true);
    try {
      const task = await submit(request);
      if (session.current === current) setPending({ id: task.id, snapshot });
    } catch (error) {
      toast.error((error as { response?: { data?: { error?: string } } }).response?.data?.error || '提交任务失败');
    } finally {
      if (session.current === current) { submitting.current = false; setSending(false); }
    }
  };
  return { start, reset, busy, task };
}
