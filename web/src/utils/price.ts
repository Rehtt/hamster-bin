/** 将元字符串或数字四舍五入换算为分 */
export function yuanToCents(yuan: string | number): number {
  const num = typeof yuan === 'string' ? parseFloat(yuan) : yuan;
  if (Number.isNaN(num) || num <= 0) return 0;
  return Math.round(num * 100);
}

/** 将分格式化为带货币符号的字符串 */
export function formatCents(cents?: number | null): string {
  if (cents == null || cents === 0) return '-';
  return `¥${(cents / 100).toFixed(2)}`;
}

/** 将分换算为元字符串（用于输入框回填） */
export function centsToYuanInput(cents?: number | null): string {
  if (cents == null || cents === 0) return '';
  return (cents / 100).toFixed(2);
}

/** 按数量分摊单价（分），整数除法 */
export function calcUnitPriceCents(totalCents: number, quantity: number): number {
  if (quantity <= 0 || totalCents <= 0) return 0;
  return Math.floor(totalCents / quantity);
}

/** 按单价与数量计算总价（分） */
export function calcTotalPriceCents(unitPriceCents: number, quantity: number): number {
  if (quantity <= 0 || unitPriceCents <= 0) return 0;
  return unitPriceCents * quantity;
}

/** 将解析层返回的元单价换算为分 */
export function parsePriceToCents(priceYuan?: number): number {
  if (priceYuan == null || priceYuan <= 0) return 0;
  return Math.round(priceYuan * 100);
}
