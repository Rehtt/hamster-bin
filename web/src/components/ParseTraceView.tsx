import type { ParseTrace } from '../types';
export function ParseTraceView({ trace }: { trace?: ParseTrace }) {
  if (!trace) return null;
  return <section className="space-y-2 text-sm" aria-label="调用记录">
    <p className="font-medium">调用记录 · {trace.mode} {trace.final_source && `· ${trace.final_source}`}</p>
    {trace.fallback && <p>已降级为普通解析：{trace.fallback_reason}</p>}
    {trace.calls?.map((call, index) => <div key={index} className="rounded border p-3 space-y-1 break-all">
      <p>{call.tool} · {call.status} · {call.duration_ms} ms{call.cached && ' · 缓存'}</p>
      <time dateTime={call.started_at}>{new Date(call.started_at).toLocaleString()}</time>
      <p>参数：{JSON.stringify(call.arguments)}</p>
      {call.endpoint && <p>查询地址：<a className="underline" href={call.endpoint} target="_blank" rel="noreferrer">{call.endpoint}</a></p>}
      <p>{call.summary}</p>
    </div>)}
  </section>;
}
