import { useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';
export function positiveInt(value: string | null, fallback = 1) { const n = Number(value); return Number.isSafeInteger(n) && n > 0 ? n : fallback; }
export function useListQuery() {
  const [params, setParams] = useSearchParams();
  const updateQuery = useCallback((patch: Record<string, string | number | undefined>, replace = false) => {
    setParams(prev => {
      const next = new URLSearchParams(prev);
      for (const [key, value] of Object.entries(patch)) { if (value === undefined || value === '') next.delete(key); else next.set(key, String(value)); }
      return next;
    }, { replace });
  }, [setParams]);
  return { params, updateQuery, page: positiveInt(params.get('page')), pageSize: [10, 20, 50, 100].includes(Number(params.get('page_size'))) ? Number(params.get('page_size')) : 20 };
}
