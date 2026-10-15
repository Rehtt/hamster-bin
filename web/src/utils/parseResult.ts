// 只回填提交后未修改的字段；空解析值保留当前值。
export function mergeParsed<T extends object>(current: T, parsed: Partial<T>, snapshot?: T): T {
  const next = { ...current };
  for (const key of Object.keys(parsed) as (keyof T)[]) {
    const value = parsed[key];
    if (value !== '' && value !== undefined && value !== null && (!snapshot || current[key] === snapshot[key])) next[key] = value as T[keyof T];
  }
  return next;
}
