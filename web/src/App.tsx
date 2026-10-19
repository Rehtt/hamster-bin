import { lazy, Suspense } from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { Toaster } from 'react-hot-toast';
import { AuthProvider } from './context/AuthContext';
import ProtectedRoute from './components/ProtectedRoute';
import Layout from './components/Layout';
import { ThemeProvider } from './context/ThemeProvider';
import { ConfirmProvider } from './components/ui/ConfirmProvider';
import { TaskProvider } from './context/TaskContext';

const Login = lazy(() => import('./pages/Login'));
const Dashboard = lazy(() => import('./pages/Dashboard'));
const Components = lazy(() => import('./pages/Components'));
const PreStocks = lazy(() => import('./pages/PreStocks'));
const Categories = lazy(() => import('./pages/Categories'));
const Tasks = lazy(() => import('./pages/Tasks'));
const StockLogs = lazy(() => import('./pages/StockLogs'));

function PageLoader() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background text-muted-foreground">
      加载中...
    </div>
  );
}

function App() {
  return (
    <ThemeProvider><ConfirmProvider><AuthProvider>
      <Router>
        <Suspense fallback={<PageLoader />}>
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route
              path="/*"
              element={
                <ProtectedRoute>
                  <TaskProvider><Layout>
                    <Routes>
                      <Route path="/" element={<Dashboard />} />
                      <Route path="/components" element={<Components />} />
                      <Route path="/pre-stocks" element={<PreStocks />} />
                      <Route path="/categories" element={<Categories />} />
                      <Route path="/tasks" element={<Tasks />} />
                      <Route path="/logs" element={<StockLogs />} />
                    </Routes>
                  </Layout></TaskProvider>
                </ProtectedRoute>
              }
            />
          </Routes>
        </Suspense>
        <Toaster position="top-right" toastOptions={{ style: { background: 'hsl(var(--card))', color: 'hsl(var(--foreground))', border: '1px solid hsl(var(--border))' } }} />
      </Router>
    </AuthProvider></ConfirmProvider></ThemeProvider>
  );
}

export default App;
