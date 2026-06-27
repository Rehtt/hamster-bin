import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Loader2, Minus, Plus, Search, Trash2 } from 'lucide-react';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import { type BatchStockOutFailure, type Component } from '../types';
import { Button } from './ui/Button';
import { Input } from './ui/Input';
import { Label } from './ui/Label';
import { Modal } from './ui/Modal';
import { QuantityShortcuts } from './ui/QuantityShortcuts';
import { calcOutboundCostCents, formatCents, formatMicro } from '../utils/price';
import { cn } from '../utils/cn';

type BatchRow = {
  component: Component;
  quantity: number;
};

type BatchStockOutModalProps = {
  isOpen: boolean;
  onClose: () => void;
  initialComponents?: Component[];
  onSuccess: () => void;
};

function componentLabel(component: Component): string {
  const parts = [
    component.component_number,
    component.name,
    component.model,
  ].filter(Boolean);
  return parts.join(' · ');
}

function supplierName(component: Component): string {
  return component.supplier?.name || '—';
}

function supplierPartNumber(component: Component): string {
  return component.supplier_part_number || '—';
}

function componentMetaLine(component: Component, extra?: string): string {
  const parts = [
    supplierName(component),
    supplierPartNumber(component),
    `库存 ${component.stock_quantity}`,
    extra,
  ].filter(Boolean);
  return parts.join(' · ');
}

export function BatchStockOutModal({
  isOpen,
  onClose,
  initialComponents = [],
  onSuccess,
}: BatchStockOutModalProps) {
  const [rows, setRows] = useState<BatchRow[]>([]);
  const [reason, setReason] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<Component[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [showSearchDropdown, setShowSearchDropdown] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [failures, setFailures] = useState<BatchStockOutFailure[]>([]);
  const searchTimerRef = useRef<number | null>(null);

  const resetForm = useCallback((components: Component[]) => {
    setRows(components.map(component => ({ component, quantity: 1 })));
    setReason('');
    setSearchQuery('');
    setSearchResults([]);
    setShowSearchDropdown(false);
    setFailures([]);
  }, []);

  useEffect(() => {
    if (isOpen) {
      resetForm(initialComponents);
    }
  }, [isOpen, initialComponents, resetForm]);

  useEffect(() => {
    if (!isOpen) return;
    const query = searchQuery.trim();
    if (searchTimerRef.current) {
      window.clearTimeout(searchTimerRef.current);
    }
    if (!query) {
      setSearchResults([]);
      setIsSearching(false);
      return;
    }
    setIsSearching(true);
    searchTimerRef.current = window.setTimeout(async () => {
      try {
        const res = await client.get('/components', {
          params: { page: 1, page_size: 10, keyword: query },
        });
        setSearchResults(res.data.data || []);
      } catch {
        setSearchResults([]);
      } finally {
        setIsSearching(false);
      }
    }, 300);
    return () => {
      if (searchTimerRef.current) {
        window.clearTimeout(searchTimerRef.current);
      }
    };
  }, [isOpen, searchQuery]);

  const failureMap = useMemo(() => {
    const map = new Map<number, BatchStockOutFailure>();
    for (const failure of failures) {
      map.set(failure.component_id, failure);
    }
    return map;
  }, [failures]);

  const validRows = useMemo(
    () => rows.filter(row => row.quantity > 0),
    [rows],
  );

  const summary = useMemo(() => {
    let totalQuantity = 0;
    let totalCostCents = 0;
    for (const row of validRows) {
      totalQuantity += row.quantity;
      if ((row.component.unit_price_micro ?? 0) > 0) {
        totalCostCents += calcOutboundCostCents(row.component.unit_price_micro!, row.quantity);
      }
    }
    return { rowCount: validRows.length, totalQuantity, totalCostCents };
  }, [validRows]);

  const addComponent = (component: Component) => {
    if (rows.some(row => row.component.id === component.id)) {
      toast.error('该元件已在列表中');
      return;
    }
    setRows(prev => [...prev, { component, quantity: 1 }]);
    setSearchQuery('');
    setSearchResults([]);
    setShowSearchDropdown(false);
    setFailures(prev => prev.filter(item => item.component_id !== component.id));
  };

  const updateQuantity = (componentId: number, quantity: number) => {
    setRows(prev =>
      prev.map(row =>
        row.component.id === componentId
          ? { ...row, quantity: Math.max(0, quantity) }
          : row,
      ),
    );
    setFailures(prev => prev.filter(item => item.component_id !== componentId));
  };

  const removeRow = (componentId: number) => {
    setRows(prev => prev.filter(row => row.component.id !== componentId));
    setFailures(prev => prev.filter(item => item.component_id !== componentId));
  };

  const handleSubmit = async () => {
    if (validRows.length === 0) {
      toast.error('请至少填写一行有效出库数量');
      return;
    }
    setIsSubmitting(true);
    setFailures([]);
    try {
      await client.post('/components/batch-stock-out', {
        reason,
        items: validRows.map(row => ({
          component_id: row.component.id,
          quantity: row.quantity,
        })),
      });
      toast.success(`批量出库成功，共 ${summary.totalQuantity} 件`);
      onSuccess();
      onClose();
    } catch (error) {
      const err = error as {
        response?: { data?: { error?: string; failures?: BatchStockOutFailure[] } };
      };
      const responseFailures = err.response?.data?.failures;
      if (responseFailures?.length) {
        setFailures(responseFailures);
        toast.error(err.response?.data?.error || '批量出库失败');
      } else {
        toast.error(err.response?.data?.error || '批量出库失败');
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="批量出库"
      className="max-w-4xl"
      footer={
        <>
          <Button variant="outline" onClick={onClose} disabled={isSubmitting}>
            取消
          </Button>
          <Button onClick={() => void handleSubmit()} disabled={isSubmitting || validRows.length === 0}>
            {isSubmitting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                提交中...
              </>
            ) : (
              '确认出库'
            )}
          </Button>
        </>
      }
    >
      <div className="space-y-4 py-2">
        <div className="space-y-2">
          <Label>添加元件</Label>
          <div className="relative">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={searchQuery}
              onChange={e => {
                setSearchQuery(e.target.value);
                setShowSearchDropdown(true);
              }}
              onFocus={() => setShowSearchDropdown(true)}
              onBlur={() => setTimeout(() => setShowSearchDropdown(false), 200)}
              placeholder="搜索编号、名称、型号..."
              className="pl-9"
            />
            {showSearchDropdown && searchQuery.trim() && (
              <div className="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-md border bg-popover shadow-lg">
                {isSearching ? (
                  <div className="px-3 py-2 text-sm text-muted-foreground">搜索中...</div>
                ) : searchResults.length === 0 ? (
                  <div className="px-3 py-2 text-sm text-muted-foreground">无匹配元件</div>
                ) : (
                  searchResults.map(component => {
                    const alreadyAdded = rows.some(row => row.component.id === component.id);
                    return (
                      <button
                        key={component.id}
                        type="button"
                        className={cn(
                          'flex w-full flex-col items-start px-3 py-2 text-left text-sm hover:bg-accent',
                          alreadyAdded && 'cursor-not-allowed opacity-50',
                        )}
                        disabled={alreadyAdded}
                        onMouseDown={e => e.preventDefault()}
                        onClick={() => addComponent(component)}
                      >
                        <span className="font-medium">{componentLabel(component)}</span>
                        <span className="text-xs text-muted-foreground">
                          {componentMetaLine(component, alreadyAdded ? '已添加' : undefined)}
                        </span>
                      </button>
                    );
                  })
                )}
              </div>
            )}
          </div>
        </div>

        {rows.length === 0 ? (
          <div className="rounded-md border border-dashed px-4 py-10 text-center text-sm text-muted-foreground">
            请搜索添加元件，或从列表勾选后打开此弹窗
          </div>
        ) : (
          <div className="max-h-[45vh] overflow-auto rounded-md border">
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-muted/80 backdrop-blur-sm">
                <tr className="border-b">
                  <th className="px-3 py-2 text-left font-medium text-muted-foreground">元件</th>
                  <th className="px-3 py-2 text-left font-medium text-muted-foreground whitespace-nowrap">供应商</th>
                  <th className="px-3 py-2 text-left font-medium text-muted-foreground whitespace-nowrap">供应商料号</th>
                  <th className="px-3 py-2 text-left font-medium text-muted-foreground whitespace-nowrap">库存</th>
                  <th className="px-3 py-2 text-left font-medium text-muted-foreground whitespace-nowrap">出库数量</th>
                  <th className="px-3 py-2 text-left font-medium text-muted-foreground whitespace-nowrap">预估成本</th>
                  <th className="px-3 py-2 text-right font-medium text-muted-foreground w-10" />
                </tr>
              </thead>
              <tbody>
                {rows.map(row => {
                  const failure = failureMap.get(row.component.id);
                  const hasPrice = (row.component.unit_price_micro ?? 0) > 0;
                  return (
                    <tr
                      key={row.component.id}
                      className={cn(
                        'border-b align-top',
                        failure && 'bg-destructive/5',
                      )}
                    >
                      <td className="px-3 py-3">
                        <div className="font-medium">{componentLabel(row.component)}</div>
                        {failure && (
                          <div className="mt-1 text-xs text-destructive">
                            {failure.error}
                            {failure.stock_quantity != null ? `（当前库存 ${failure.stock_quantity}）` : ''}
                          </div>
                        )}
                      </td>
                      <td className="px-3 py-3 whitespace-nowrap">{supplierName(row.component)}</td>
                      <td className="px-3 py-3 whitespace-nowrap">{supplierPartNumber(row.component)}</td>
                      <td className="px-3 py-3 whitespace-nowrap">{row.component.stock_quantity}</td>
                      <td className="px-3 py-3">
                        <div className="flex min-w-[160px] items-center gap-2">
                          <Button
                            type="button"
                            variant="outline"
                            size="icon"
                            className="h-8 w-8 shrink-0"
                            onClick={() => updateQuantity(row.component.id, row.quantity - 1)}
                          >
                            <Minus className="h-4 w-4" />
                          </Button>
                          <Input
                            type="number"
                            min="0"
                            value={row.quantity}
                            onChange={e => updateQuantity(row.component.id, Number(e.target.value))}
                            className="h-8 text-center"
                          />
                          <Button
                            type="button"
                            variant="outline"
                            size="icon"
                            className="h-8 w-8 shrink-0"
                            onClick={() => updateQuantity(row.component.id, row.quantity + 1)}
                          >
                            <Plus className="h-4 w-4" />
                          </Button>
                        </div>
                        <div className="mt-2">
                          <QuantityShortcuts
                            value={row.quantity}
                            onSelect={quantity => updateQuantity(row.component.id, quantity)}
                          />
                        </div>
                      </td>
                      <td className="px-3 py-3 whitespace-nowrap">
                        {hasPrice && row.quantity > 0 ? (
                          <div className="space-y-1">
                            <div>{formatCents(calcOutboundCostCents(row.component.unit_price_micro!, row.quantity))}</div>
                            <div className="text-xs text-muted-foreground">
                              单价 {formatMicro(row.component.unit_price_micro)}
                            </div>
                          </div>
                        ) : (
                          <span className="text-muted-foreground">—</span>
                        )}
                      </td>
                      <td className="px-3 py-3 text-right">
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-muted-foreground hover:text-destructive"
                          onClick={() => removeRow(row.component.id)}
                          aria-label="移除"
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        <div className="rounded-md border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
          <div>有效行数：{summary.rowCount}</div>
          <div>总出库数量：{summary.totalQuantity}</div>
          {summary.totalCostCents > 0 && (
            <div>总预估成本：{formatCents(summary.totalCostCents)}</div>
          )}
        </div>

        <div className="space-y-2">
          <Label>备注</Label>
          <Input value={reason} onChange={e => setReason(e.target.value)} placeholder="出库原因，如项目装配" />
        </div>
      </div>
    </Modal>
  );
}
