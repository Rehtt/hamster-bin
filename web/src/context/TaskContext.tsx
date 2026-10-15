import { useCallback, useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import type { ParseTask, TaskRequest } from '../types';
import { TaskContext, taskPending, taskStatusLabel } from './tasks';

const STORAGE_KEY = 'hamster-pending-tasks';
function savedIDs(): number[] {
  try { const value: unknown = JSON.parse(sessionStorage.getItem(STORAGE_KEY) || '[]'); return Array.isArray(value) ? value.filter(id => Number.isSafeInteger(id) && id > 0) : []; } catch { return []; }
}

export function TaskProvider({ children }: { children: React.ReactNode }) {
  const [tasks, setTasks] = useState<Record<number, ParseTask>>({});
  const pending = useRef(new Set<number>(savedIDs()));
  const notified = useRef(new Set<number>());
  const persist = useCallback(() => {
    try { sessionStorage.setItem(STORAGE_KEY, JSON.stringify([...pending.current])); } catch { /* 禁用存储时仍可轮询 */ }
  }, []);
  const track = useCallback((task: ParseTask) => {
    if (notified.current.has(task.id)) return;
    if (taskPending(task)) pending.current.add(task.id);
    setTasks(prev => ({ ...prev, [task.id]: task }));
    persist();
  }, [persist]);
  const submit = useCallback(async (request: TaskRequest) => {
    const res = await client.post('/tasks', request);
    const task = res.data.data as ParseTask;
    track(task);
    toast.success(`已加入任务 #${task.id}`);
    return task;
  }, [track]);

  useEffect(() => {
    const controller = new AbortController();
    let busy = false;
    let restored = false;
    const restore = async () => {
      for (const status of ['queued', 'running']) {
        let page = 1;
        while (!controller.signal.aborted) {
          const res = await client.get('/tasks', { params: { status, page, page_size: 100 }, signal: controller.signal });
          if (controller.signal.aborted) return;
          for (const task of res.data.data as ParseTask[]) track(task);
          if (page >= res.data.pagination.total_page) break;
          page++;
        }
      }
    };
    const poll = async () => {
      if (busy || controller.signal.aborted) return;
      busy = true;
      try {
        if (!restored) { await restore(); restored = true; }
        await Promise.all([...pending.current].map(async id => {
          try {
            const res = await client.get(`/tasks/${id}`, { signal: controller.signal });
            if (controller.signal.aborted) return;
            const task = res.data.data as ParseTask;
            setTasks(prev => ({ ...prev, [id]: task }));
            if (!taskPending(task) && pending.current.delete(id)) {
              persist();
              if (notified.current.has(id)) return;
              notified.current.add(id);
              toast(t => <span>任务 #{id} {taskStatusLabel[task.status]} · <Link className="underline" to={`/tasks?task=${id}`} onClick={() => toast.dismiss(t.id)}>查看结果</Link></span>, { duration: 7000 });
            }
          } catch (error) {
            if ((error as { response?: { status?: number } }).response?.status === 404) { pending.current.delete(id); persist(); }
          }
        }));
      } catch { /* 网络恢复后下次轮询重试初始化 */ }
      finally { busy = false; }
    };
    void poll();
    const timer = window.setInterval(() => void poll(), 2000);
    return () => { controller.abort(); window.clearInterval(timer); };
  }, [persist, track]);

  return <TaskContext.Provider value={{ tasks, submit, track }}>{children}</TaskContext.Provider>;
}
