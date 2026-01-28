import { useEffect, useState } from 'react';
import { Plus, Pencil, Trash2 } from 'lucide-react';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import { type Category } from '../types';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { Label } from '../components/ui/Label';
import { Card, CardContent } from '../components/ui/Card';

export default function Categories() {
  const [categories, setCategories] = useState<Category[]>([]);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingCategory, setEditingCategory] = useState<Category | null>(null);
  const [formData, setFormData] = useState({ name: '' });

  const fetchCategories = async () => {
    try {
      const res = await client.get('/categories');
      setCategories(res.data.data || []);
    } catch (error) {
      toast.error('加载分类失败');
    }
  };

  useEffect(() => {
    fetchCategories();
  }, []);

  const handleSubmit = async () => {
    if (!formData.name) return toast.error('请输入分类名称');
    
    try {
      if (editingCategory) {
        await client.put(`/categories/${editingCategory.id}`, formData);
        toast.success('更新成功');
      } else {
        await client.post('/categories', formData);
        toast.success('添加成功');
      }
      setIsModalOpen(false);
      fetchCategories();
    } catch (error) {
      toast.error('操作失败');
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('确定删除此分类吗？')) return;
    try {
      await client.delete(`/categories/${id}`);
      toast.success('删除成功');
      fetchCategories();
    } catch (error) {
      toast.error('删除失败');
    }
  };

  const openModal = (category?: Category) => {
    if (category) {
      setEditingCategory(category);
      setFormData({ name: category.name });
    } else {
      setEditingCategory(null);
      setFormData({ name: '' });
    }
    setIsModalOpen(true);
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-3xl font-bold tracking-tight">分类管理</h2>
        <Button onClick={() => openModal()}>
          <Plus className="mr-2 h-4 w-4" /> 添加分类
        </Button>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {categories.map((category) => (
          <Card key={category.id}>
            <CardContent className="flex justify-between items-center p-6">
              <span className="font-medium">{category.name}</span>
              <div className="space-x-2">
                <Button variant="ghost" size="icon" onClick={() => openModal(category)}>
                  <Pencil className="h-4 w-4" />
                </Button>
                <Button variant="ghost" size="icon" className="text-destructive hover:text-destructive" onClick={() => handleDelete(category.id)}>
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      <Modal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        title={editingCategory ? '编辑分类' : '添加分类'}
        footer={
          <>
            <Button variant="outline" onClick={() => setIsModalOpen(false)}>取消</Button>
            <Button onClick={handleSubmit}>保存</Button>
          </>
        }
      >
        <div className="space-y-2">
          <Label htmlFor="name">分类名称</Label>
          <Input
            id="name"
            value={formData.name}
            onChange={(e) => setFormData({ name: e.target.value })}
            placeholder="例如: 电阻, 电容"
          />
        </div>
      </Modal>
    </div>
  );
}
