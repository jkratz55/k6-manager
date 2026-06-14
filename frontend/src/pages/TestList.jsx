import React, { useState, useEffect, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { getTests, deleteTest, rerunTest, stopTest } from '../api/client';
import { 
  Search, 
  Filter, 
  ArrowUpDown, 
  MoreVertical, 
  Trash2, 
  Eye, 
  RefreshCcw,
  Clock,
  CheckCircle2,
  AlertCircle,
  Loader2,
  Play,
  Square
} from 'lucide-react';
import { format } from 'date-fns';
import Fuse from 'fuse.js';

const StatusBadge = ({ status }) => {
  switch (status?.toLowerCase()) {
    case 'succeeded':
    case 'completed':
    case 'finished':
      return <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800"><CheckCircle2 size={12} /> Succeeded</span>;
    case 'failed':
    case 'error':
    case 'errored':
      return <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800"><AlertCircle size={12} /> Failed</span>;
    case 'running':
    case 'started':
    case 'created':
    case 'initialization':
      return <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800"><Loader2 size={12} className="animate-spin" /> {status}</span>;
    default:
      return <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-slate-100 text-slate-800"><Clock size={12} /> {status || 'Unknown'}</span>;
  }
};

const TestList = () => {
  const [tests, setTests] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [sortConfig, setSortConfig] = useState({ key: 'startedAt', direction: 'desc' });

  const fetchTests = async () => {
    setLoading(true);
    try {
      const data = await getTests();
      setTests(data || []);
      setError(null);
    } catch (err) {
      setError('Failed to fetch tests');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTests();
  }, []);

  const handleDelete = async (id) => {
    if (window.confirm('Are you sure you want to delete this test?')) {
      try {
        await deleteTest(id);
        setTests(tests.filter(t => t.id !== id));
      } catch (err) {
        alert('Failed to delete test');
      }
    }
  };

  const handleReRun = async (id) => {
    try {
      await rerunTest(id);
      fetchTests(); // Refresh the list
    } catch (err) {
      alert('Failed to re-run test');
      console.error(err);
    }
  };

  const handleStop = async (id) => {
    if (window.confirm('Are you sure you want to stop this test?')) {
      try {
        await stopTest(id);
        fetchTests(); // Refresh the list
      } catch (err) {
        alert('Failed to stop test');
        console.error(err);
      }
    }
  };

  const isFinished = (status) => {
    switch (status?.toLowerCase()) {
      case 'succeeded':
      case 'completed':
      case 'finished':
      case 'failed':
      case 'error':
      case 'errored':
        return true;
      default:
        return false;
    }
  };

  const filteredAndSortedTests = useMemo(() => {
    let result = [...tests];

    // Status filtering
    if (statusFilter !== 'all') {
      result = result.filter(t => t.phase?.toLowerCase() === statusFilter.toLowerCase());
    }

    // Fuzzy Search
    if (searchQuery) {
      const fuse = new Fuse(result, {
        keys: ['name', 'id'],
        threshold: 0.3
      });
      result = fuse.search(searchQuery).map(r => r.item);
    }

    // Sorting
    result.sort((a, b) => {
      if (!a[sortConfig.key]) return 1;
      if (!b[sortConfig.key]) return -1;
      
      let aVal = a[sortConfig.key];
      let bVal = b[sortConfig.key];

      if (sortConfig.key === 'startedAt' || sortConfig.key === 'finishedAt') {
        aVal = new Date(aVal).getTime();
        bVal = new Date(bVal).getTime();
      }

      if (aVal < bVal) return sortConfig.direction === 'asc' ? -1 : 1;
      if (aVal > bVal) return sortConfig.direction === 'asc' ? 1 : -1;
      return 0;
    });

    return result;
  }, [tests, searchQuery, statusFilter, sortConfig]);

  const requestSort = (key) => {
    let direction = 'asc';
    if (sortConfig.key === key && sortConfig.direction === 'asc') {
      direction = 'desc';
    }
    setSortConfig({ key, direction });
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-end">
        <div>
          <h1 className="text-3xl font-bold text-slate-900">Test Runs</h1>
          <p className="text-slate-500">Manage and monitor your k6 load tests</p>
        </div>
        <button 
          onClick={fetchTests}
          className="btn btn-outline gap-2"
          disabled={loading}
        >
          <RefreshCcw size={16} className={loading ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      <div className="card p-4">
        <div className="flex flex-col md:flex-row gap-4">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={18} />
            <input 
              type="text" 
              placeholder="Search by name or ID..." 
              className="input pl-10"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </div>
          <div className="flex gap-4">
            <div className="relative min-w-[150px]">
              <Filter className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={18} />
              <select 
                className="input pl-10 appearance-none"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
              >
                <option value="all">All Statuses</option>
                <option value="Running">Running</option>
                <option value="Succeeded">Succeeded</option>
                <option value="Failed">Failed</option>
              </select>
            </div>
          </div>
        </div>
      </div>

      {loading && tests.length === 0 ? (
        <div className="flex justify-center py-12">
          <Loader2 className="animate-spin text-blue-600" size={48} />
        </div>
      ) : error ? (
        <div className="bg-red-50 border border-red-200 text-red-700 p-4 rounded-md">
          {error}
        </div>
      ) : filteredAndSortedTests.length === 0 ? (
        <div className="card py-12 text-center">
          <p className="text-slate-500">No tests found matching your criteria</p>
        </div>
      ) : (
        <div className="card overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="bg-slate-50 border-b border-slate-200">
                  <th className="px-6 py-4 text-sm font-semibold text-slate-900">
                    <button onClick={() => requestSort('name')} className="flex items-center gap-1 hover:text-blue-600">
                      Name <ArrowUpDown size={14} />
                    </button>
                  </th>
                  <th className="px-6 py-4 text-sm font-semibold text-slate-900">
                    <button onClick={() => requestSort('phase')} className="flex items-center gap-1 hover:text-blue-600">
                      Status <ArrowUpDown size={14} />
                    </button>
                  </th>
                  <th className="px-6 py-4 text-sm font-semibold text-slate-900">
                    <button onClick={() => requestSort('startedAt')} className="flex items-center gap-1 hover:text-blue-600">
                      Started <ArrowUpDown size={14} />
                    </button>
                  </th>
                  <th className="px-6 py-4 text-sm font-semibold text-slate-900 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200">
                {filteredAndSortedTests.map((test) => (
                  <tr key={test.id} className="hover:bg-slate-50 transition-colors">
                    <td className="px-6 py-4">
                      <div className="font-medium text-slate-900">{test.name}</div>
                      <div className="text-xs text-slate-500 font-mono">{test.id}</div>
                    </td>
                    <td className="px-6 py-4">
                      <StatusBadge status={test.phase} />
                    </td>
                    <td className="px-6 py-4 text-sm text-slate-600">
                      {test.startedAt ? format(new Date(test.startedAt), 'MMM d, HH:mm:ss') : '-'}
                    </td>
                    <td className="px-6 py-4 text-right">
                      <div className="flex justify-end gap-2">
                        <Link to={`/tests/${test.id}`} className="p-2 text-slate-400 hover:text-blue-600 transition-colors" title="View Details">
                          <Eye size={18} />
                        </Link>
                        <button 
                          onClick={() => handleReRun(test.id)}
                          className="p-2 text-slate-400 hover:text-green-600 transition-colors"
                          title="Re-Run"
                        >
                          <Play size={18} />
                        </button>
                        {!isFinished(test.phase) && (
                          <button 
                            onClick={() => handleStop(test.id)}
                            className="p-2 text-slate-400 hover:text-orange-600 transition-colors"
                            title="Stop Test"
                          >
                            <Square size={18} />
                          </button>
                        )}
                        <button 
                          onClick={() => handleDelete(test.id)}
                          className="p-2 text-slate-400 hover:text-red-600 transition-colors"
                          title="Delete Test"
                        >
                          <Trash2 size={18} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
};

export default TestList;
