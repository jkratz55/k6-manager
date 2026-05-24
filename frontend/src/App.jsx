import React from 'react';
import { BrowserRouter as Router, Routes, Route, Link, useNavigate } from 'react-router-dom';
import { Layout, List, Plus, Activity } from 'lucide-react';
import TestList from './pages/TestList';
import TestDetail from './pages/TestDetail';
import CreateTest from './pages/CreateTest';

const AppLayout = ({ children }) => {
  return (
    <div className="min-h-screen flex flex-col">
      <header className="bg-white border-b border-slate-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16 items-center">
            <div className="flex items-center">
              <Link to="/" className="flex items-center gap-2 text-blue-600 font-bold text-xl">
                <Activity size={24} />
                <span>k6 Manager</span>
              </Link>
            </div>
            <nav className="flex gap-4">
              <Link to="/" className="text-slate-600 hover:text-slate-900 px-3 py-2 text-sm font-medium">Tests</Link>
              <Link to="/create" className="btn btn-primary gap-2">
                <Plus size={16} />
                <span>New Test</span>
              </Link>
            </nav>
          </div>
        </div>
      </header>
      <main className="flex-1 max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 w-full">
        {children}
      </main>
      <footer className="bg-white border-t border-slate-200 py-6">
        <div className="max-w-7xl mx-auto px-4 text-center text-slate-500 text-sm">
          &copy; {new Date().getFullYear()} k6 Manager - Professional Load Testing
        </div>
      </footer>
    </div>
  );
};

function App() {
  return (
    <Router>
      <AppLayout>
        <Routes>
          <Route path="/" element={<TestList />} />
          <Route path="/tests/:id" element={<TestDetail />} />
          <Route path="/create" element={<CreateTest />} />
        </Routes>
      </AppLayout>
    </Router>
  );
}

export default App;
