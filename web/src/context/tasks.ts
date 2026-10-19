import { createContext, useContext } from 'react';
import type { ParseTask, TaskRequest } from '../types';

export const taskStatusLabel: Record<ParseTask['status'], string> = {
  queued: '排队中', running: '运行中', succeeded: '已完成', failed: '失败', cancelled: '已取消',
};
export const taskPending = (task?: ParseTask) => task?.status === 'queued' || task?.status === 'running';
export const TaskContext = createContext<{
  tasks: Record<number, ParseTask>;
  submit: (request: TaskRequest) => Promise<ParseTask>;
  submitImage: (file: File, source: TaskRequest['source'], recordID?: number) => Promise<ParseTask>;
  track: (task: ParseTask) => void;
  forget: (ids: number[]) => void;
} | null>(null);
export function useTasks() {
  const value = useContext(TaskContext);
  if (!value) throw new Error('缺少 TaskProvider');
  return value;
}
