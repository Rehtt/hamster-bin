import type { ParseTask } from '../types';
import { taskPending } from '../context/tasks';

export type TaskDeleteMode = 'selected' | 'failed' | 'all' | 'older_than_7_days';
export interface TaskDeleteResult {
  deleted_ids: number[];
  deleted_count: number;
  skipped_count: number;
  image_cleanup_failed_count: number;
}

// 删除和查询范围改变后，之前发出的响应不得恢复旧列表或详情。
export class TaskResponseGuard {
  private revision = 0;
  private deleted = new Set<number>();
  snapshot() { return this.revision; }
  invalidate() { this.revision++; }
  accepts(revision: number) { return revision === this.revision; }
  has(id: number) { return this.deleted.has(id); }
  forget(ids: number[]) { ids.forEach(id => this.deleted.add(id)); this.invalidate(); }
}

export function pruneTaskSelection(ids: number[], items: ParseTask[]) {
  const valid = new Set(items.filter(task => !taskPending(task)).map(task => task.id));
  return ids.filter(id => valid.has(id));
}
export const lastTaskPage = (total: number, size: number) => Math.max(1, Math.ceil(total / size));
export function taskDeleteConfirmation(mode: TaskDeleteMode, ids: number[], single = false) {
  const scope = single ? `任务 #${ids[0]}` : mode === 'selected' ? `选中的 ${ids.length} 项任务` : {
    failed: '全库失败任务（仅 failed）', all: '全库任务', older_than_7_days: '全库创建时间早于服务器当前时间减去 168 小时的任务',
  }[mode];
  return `确定删除${scope}？${mode !== 'selected' ? '此操作覆盖全部分页与筛选。' : ''}仅删除已完成、失败、已取消的任务，排队和运行中的任务保留。删除不可恢复，记录及结果将永久删除；原图仅在没有其他任务引用时清理，已有元件、预入库及重试任务不受影响。`;
}
