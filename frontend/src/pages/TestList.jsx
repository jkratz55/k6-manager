import { useState, useMemo, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { getTests, deleteTest, rerunTest, stopTest } from '../api/client';
import {
  Search,
  Filter,
  ArrowUpDown,
  Trash2,
  Eye,
  RefreshCcw,
  Loader2,
  Play,
  Square
} from 'lucide-react';
import { format } from 'date-fns';
import Fuse from 'fuse.js';
import StatusBadge from '../components/StatusBadge';
import { STATUS_FILTERS, errorMessage, testStatus } from '../lib/status';
import { useAutoRefresh } from '../lib/useAutoRefresh';

// Poll quickly while any test is in flight, and slowly otherwise so tests
// started elsewhere still show up.
const ACTIVE_POLL_MS = 5000;
const IDLE_POLL_MS = 30000;

const TestList = () => {
  const [tests, setTests] = useState([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [sortConfig, setSortConfig] = useState({ key: 'startedAt', direction: 'desc' });

  const loadTests = useCallback(async () => {
    try {
      const data = await getTests();
      setTests(data || []);
      setError(null);
    } catch (err) {
      setError(errorMessage(err, 'Failed to fetch tests'));
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, []);

  const hasActiveTests = tests.some(t => !t.finished);
  useAutoRefresh(loadTests, hasActiveTests ? ACTIVE_POLL_MS : IDLE_POLL_MS);

  const handleRefresh = async () => {
    setRefreshing(true);
    await loadTests();
    setRefreshing(false);
  };

  const handleDelete = async (id) => {
    if (window.confirm('Are you sure you want to delete this test?')) {
      try {
        await deleteTest(id);
        setTests(prev => prev.filter(t => t.id !== id));
      } catch (err) {
        alert(errorMessage(err, 'Failed to delete test'));
        console.error(err);
      }
    }
  };

  const handleReRun = async (id) => {
    try {
      await rerunTest(id);
      loadTests();
    } catch (err) {
      alert(errorMessage(err, 'Failed to re-run test'));
      console.error(err);
    }
  };

  const handleStop = async (id) => {
    if (window.confirm('Are you sure you want to stop this test?')) {
      try {
        await stopTest(id);
        loadTests();
      } catch (err) {
        alert(errorMessage(err, 'Failed to stop test'));
        console.error(err);
      }
    }
  };

  const filteredAndSortedTests = useMemo(() => {
    let result = [...tests];

    // Status filtering
    if (statusFilter !== 'all') {
      result = result.filter(t => testStatus(t).category === statusFilter);
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
          onClick={handleRefresh}
          className="btn btn-outline gap-2"
          disabled={loading || refreshing}
        >
          <RefreshCcw size={16} className={refreshing ? 'animate-spin' : ''} />
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
                {STATUS_FILTERS.map(f => (
                  <option key={f.value} value={f.value}>{f.label}</option>
                ))}
              </select>
            </div>
          </div>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 p-4 rounded-md">
          {error}
        </div>
      )}

      {loading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="animate-spin text-blue-600" size={48} />
        </div>
      ) : filteredAndSortedTests.length === 0 ? (
        !error && (
          <div className="card py-12 text-center">
            <p className="text-slate-500">No tests found matching your criteria</p>
          </div>
        )
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
                      <StatusBadge test={test} />
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
                        {!test.finished && (
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
