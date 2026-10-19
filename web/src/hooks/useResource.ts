import { useCallback, useEffect, useState } from 'react';
import client from '../api/client';
// 请求结果与参数绑定；参数变化立即进入加载态，旧请求不能覆盖新页面。
export function useResource<T>(url: string) {
  const [revision, setRevision] = useState(0);
  const key = `${url}:${revision}`;
  const [result, setResult] = useState<{ key: string; data?: T; error: string }>();
  const refresh = useCallback(() => setRevision(value => value + 1), []);
  useEffect(() => {
    const controller = new AbortController();
    void client.get<T>(url, { signal: controller.signal }).then(res => {
      if (!controller.signal.aborted) setResult({ key, data: res.data, error: '' });
    }).catch(() => {
      if (!controller.signal.aborted) setResult({ key, error: '数据加载失败' });
    });
    return () => controller.abort();
  }, [url, key]);
  return { data: result?.key === key ? result.data : undefined, loading: result?.key !== key, error: result?.key === key ? result.error : '', refresh };
}
