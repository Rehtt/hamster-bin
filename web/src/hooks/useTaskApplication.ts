import { useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import type { ParseTask, TaskRequest } from '../types';

export function useTaskApplication<T>(source: TaskRequest['source'], onApply: (task: ParseTask, record?: T) => void) {
  const [params, setParams] = useSearchParams();
  const taskID = params.get('applyTask');
  useEffect(() => {
    if (!taskID) return;
    let disposed = false;
    const load = async () => {
      try {
        const res = await client.get(`/tasks/${taskID}`);
        const task = res.data.data as ParseTask;
        if (task.status !== 'succeeded' || !task.result || (task.source && task.source !== source)) throw new Error('此任务没有可应用的结果');
        let record: T | undefined;
        if (task.record_id) {
          try {
            const current = await client.get(`/${source}/${task.record_id}`);
            if (source === 'pre-stocks' && current.data.data.status === 'confirmed') throw new Error('confirmed');
            record = current.data.data as T;
          } catch (error) {
            if ((error as { response?: { status?: number } }).response?.status !== 404 && (error as Error).message !== 'confirmed') throw error;
            if (disposed) return;
            if (!confirm('目标记录已删除或预入库已确认，是否将解析结果用于新增表单？')) return;
          }
        }
        if (!disposed) onApply(task, record);
      } catch (error) {
        if (!disposed) toast.error((error as Error).message || '恢复任务结果失败');
      } finally {
        if (!disposed) setParams(prev => { const next = new URLSearchParams(prev); next.delete('applyTask'); return next; }, { replace: true });
      }
    };
    void load();
    return () => { disposed = true; };
    // 仅在导航到新的任务时应用，表单编辑不触发重新加载。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [taskID, source, setParams]);
}
