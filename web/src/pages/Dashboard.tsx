import { useEffect, useState } from 'react';
import { Package, Layers, Database } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/Card';
import client from '../api/client';
import { type Component, type Category } from '../types';

export default function Dashboard() {
  const [stats, setStats] = useState({
    components: 0,
    stock: 0,
    categories: 0
  });

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [compRes, catRes] = await Promise.all([
          client.get<{ data: Component[], pagination: { total: number } }>('/components?page_size=1'),
          client.get<{ data: Category[] }>('/categories')
        ]);

        const allComponentsRes = await client.get<{ data: Component[] }>('/components?page_size=10000');
        const allComponents = allComponentsRes.data.data || [];
        const totalStock = allComponents.reduce((sum, c) => sum + c.stock_quantity, 0);

        setStats({
          components: compRes.data.pagination?.total || 0,
          categories: catRes.data.data?.length || 0,
          stock: totalStock
        });
      } catch (error) {
        console.error('Failed to fetch stats:', error);
      }
    };

    fetchData();
  }, []);

  return (
    <div className="space-y-6">
      <h2 className="text-3xl font-bold tracking-tight">仪表盘</h2>
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              元件总数
            </CardTitle>
            <Package className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats.components}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              库存总量
            </CardTitle>
            <Database className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats.stock}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              分类数量
            </CardTitle>
            <Layers className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats.categories}</div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
