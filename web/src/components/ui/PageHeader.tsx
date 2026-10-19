import type { ReactNode } from 'react';
const descriptions: Record<string, string> = {
  '仪表盘': '掌握库存全貌，让每一次取用都有迹可循。',
  '元件管理': '快速找到所需元件，轻松完成库存操作。',
  '预入库': '先记录采购，收货后确认入库。',
  '分类管理': '有序整理元件，让查找更简单。',
  '任务中心': '查看解析进度，核对并应用识别结果。',
  '库存记录': '追踪每一次入库、出库与价格变更。',
};
export function PageHeader({ title, actions, description }: { title: string; actions?: ReactNode; description?: string }) {
  return <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between"><div className="shrink-0"><h1 className="text-2xl font-semibold tracking-tight">{title}</h1><p className="mt-2 text-sm text-muted-foreground">{description || descriptions[title]}</p></div>{actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}</div>;
}
