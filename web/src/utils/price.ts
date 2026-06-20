/** 1 分 = 10,000 微元 */
export const MICRO_PER_CENT = 10000;

/** 将元字符串或数字四舍五入换算为分 */
export function yuanToCents(yuan: string | number): number {
  const num = typeof yuan === 'string' ? parseFloat(yuan) : yuan;
  if (Number.isNaN(num) || num <= 0) return 0;
  return Math.round(num * 100);
}

/** 将分格式化为带货币符号的字符串（用于总价） */
export function formatCents(cents?: number | null): string {
  if (cents == null || cents === 0) return '-';
  return `¥${(cents / 100).toFixed(2)}`;
}

/** 将微元格式化为带货币符号的字符串（用于单价，最多 6 位小数） */
export function formatMicro(micro?: number | null): string {
  if (micro == null || micro === 0) return '-';
  const yuan = micro / 1_000_000;
  let formatted = yuan.toFixed(6).replace(/\.?0+$/, '');
  if (!formatted.includes('.')) {
    formatted += '.00';
  } else {
    const [, decimals = ''] = formatted.split('.');
    if (decimals.length < 2) {
      formatted = yuan.toFixed(2);
    }
  }
  return `¥${formatted}`;
}

/** 将微元换算为元字符串（用于输入框回填） */
export function microToYuanInput(micro?: number | null): string {
  if (micro == null || micro === 0) return '';
  return (micro / 1_000_000).toFixed(6).replace(/\.?0+$/, '');
}

/** 将分换算为元字符串（用于总价输入框回填） */
export function centsToYuanInput(cents?: number | null): string {
  if (cents == null || cents === 0) return '';
  return (cents / 100).toFixed(2);
}

/** 按数量分摊单价（微元），四舍五入 */
export function calcUnitPriceMicro(totalCents: number, quantity: number): number {
  if (quantity <= 0 || totalCents <= 0) return 0;
  return Math.round((totalCents * MICRO_PER_CENT) / quantity);
}

/** 按单价（微元）与数量计算出库成本（分），四舍五入 */
export function calcOutboundCostCents(unitPriceMicro: number, quantity: number): number {
  if (quantity <= 0 || unitPriceMicro <= 0) return 0;
  return Math.round((unitPriceMicro * quantity) / MICRO_PER_CENT);
}

/** 将解析层返回的元单价换算为微元 */
export function parsePriceToMicro(priceYuan?: number): number {
  if (priceYuan == null || priceYuan <= 0) return 0;
  return Math.round(priceYuan * 1_000_000);
}
