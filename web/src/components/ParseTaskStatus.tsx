import { Link } from 'react-router-dom';
import type { ParseTask } from '../types';
import { taskStatusLabel } from '../context/tasks';
export function ParseTaskStatus({ task }: { task?: ParseTask }) {
  if (!task) return null;
  return <p className="text-sm text-muted-foreground" role="status">已加入任务 #{task.id} · {taskStatusLabel[task.status]}，可继续编辑或关闭表单。<Link className="underline ml-2" to={`/tasks?task=${task.id}`}>查看任务</Link></p>;
}
