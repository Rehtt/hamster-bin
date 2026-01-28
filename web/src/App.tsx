import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { Toaster } from 'react-hot-toast';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Components from './pages/Components';
import Categories from './pages/Categories';
import StockLogs from './pages/StockLogs';

function App() {
  return (
    <Router>
      <Layout>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/components" element={<Components />} />
          <Route path="/categories" element={<Categories />} />
          <Route path="/logs" element={<StockLogs />} />
        </Routes>
      </Layout>
      <Toaster position="top-right" />
    </Router>
  );
}

export default App;
