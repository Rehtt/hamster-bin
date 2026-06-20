import { useEffect, useState, type ComponentType } from 'react';
import {
  Package,
  Layers,
  Database,
  CircleDollarSign,
  TrendingDown,
  ArrowDownToLine,
  ArrowUpFromLine,
} from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import client from '../api/client';
import { type DashboardStats, type StatsRange } from '../types';
import { formatCents } from '../utils/price';

const rangeOptions: { value: StatsRange; label: string }[] = [
  { value: 'month', label: '本月' },
  { value: 'quarter', label: '本季' },
  { value: 'all', label: '全部' },
];

const rangeLabel: Record<StatsRange, string> = {
  month: '本月',
  quarter: '本季',
  all: '全部',
};

const emptyStats: DashboardStats = {
  range: 'month',
  range_start: null,
  range_end: null,
  component_count: 0,
  category_count: 0,
  total_stock: 0,
  inventory_value_cents: 0,
  inbound_quantity: 0,
  outbound_quantity: 0,
  inbound_cost_cents: 0,
};

export default function Dashboard() {
  const [range, setRange] = useState<StatsRange>('month');
  const [stats, setStats] = useState<DashboardStats>(emptyStats);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      try {
        const res = await client.get<{ data: DashboardStats }>(`/stats?range=${range}`);
        setStats(res.data.data ?? emptyStats);
      } catch (error) {
        console.error('Failed to fetch stats:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [range]);

  const periodLabel = rangeLabel[stats.range] ?? rangeLabel[range];

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h2 className="text-3xl font-bold tracking-tight">仪表盘</h2>
        <div className="flex gap-2">
          {rangeOptions.map((option) => (
            <Button
              key={option.value}
              size="sm"
              variant={range === option.value ? 'default' : 'outline'}
              onClick={() => setRange(option.value)}
            >
              {option.label}
            </Button>
          ))}
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="元件总数"
          value={stats.component_count}
          icon={Package}
          loading={loading}
        />
        <StatCard
          title="库存总量"
          value={stats.total_stock}
          icon={Database}
          loading={loading}
        />
        <StatCard
          title="分类数量"
          value={stats.category_count}
          icon={Layers}
          loading={loading}
        />
        <StatCard
          title="库存总价值"
          value={formatCents(stats.inventory_value_cents)}
          icon={CircleDollarSign}
          loading={loading}
        />
        <StatCard
          title="累计入库金额"
          subtitle={periodLabel}
          value={formatCents(stats.inbound_cost_cents)}
          icon={TrendingDown}
          loading={loading}
        />
        <StatCard
          title="入库数量"
          subtitle={periodLabel}
          value={stats.inbound_quantity}
          icon={ArrowDownToLine}
          loading={loading}
        />
        <StatCard
          title="出库数量"
          subtitle={periodLabel}
          value={stats.outbound_quantity}
          icon={ArrowUpFromLine}
          loading={loading}
        />
      </div>
    </div>
  );
}

function StatCard({
  title,
  subtitle,
  value,
  icon: Icon,
  loading,
}: {
  title: string;
  subtitle?: string;
  value: string | number;
  icon: ComponentType<{ className?: string }>;
  loading: boolean;
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">
          {title}
          {subtitle && (
            <span className="ml-1 text-xs font-normal text-muted-foreground">({subtitle})</span>
          )}
        </CardTitle>
        <Icon className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">
          {loading ? '...' : value}
        </div>
      </CardContent>
    </Card>
  );
}
