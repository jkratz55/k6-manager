import { useState, useCallback } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { getTest, deleteTest, rerunTest, stopTest } from '../api/client';
import {
  ArrowLeft,
  Trash2,
  FileCode,
  Server,
  Calendar,
  Activity,
  CheckCircle2,
  AlertCircle,
  Loader2,
  Play,
  Square,
  RefreshCcw,
  Settings,
  KeyRound
} from 'lucide-react';
import { format } from 'date-fns';
import CodeEditor from '../components/CodeEditor';
import StatusBadge from '../components/StatusBadge';
import { errorMessage, testStatus } from '../lib/status';
import { useAutoRefresh } from '../lib/useAutoRefresh';

const POLL_MS = 5000;

const formatTime = (value) => (value ? format(new Date(value), 'MMM d, HH:mm:ss') : '-');

const TITLE_ICONS = {
  active: <Loader2 className="text-blue-500 animate-spin" size={28} />,
  passed: <CheckCircle2 className="text-green-500" size={28} />,
  failed: <AlertCircle className="text-red-500" size={28} />,
  stopped: <Square className="text-orange-500" size={28} />,
  finished: null,
};

const Row = ({ label, children }) => (
  <div className="flex justify-between gap-4">
    <span className="text-slate-600 shrink-0">{label}</span>
    <span className="text-slate-900 text-right break-all">{children}</span>
  </div>
);

const TestDetailPage = ({ id }) => {
  const navigate = useNavigate();
  const [test, setTest] = useState(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState(null);

  const loadTest = useCallback(async () => {
    try {
      const data = await getTest(id);
      setTest(data);
      setError(null);
    } catch (err) {
      if (err.response?.status === 404) {
        setTest(null);
        setError('Test not found. It may have been deleted.');
      } else {
        setError(errorMessage(err, 'Failed to fetch test details'));
      }
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [id]);

  // Keep the status current until the runners are done.
  useAutoRefresh(loadTest, POLL_MS, Boolean(test) && !test.finished);

  const handleRefresh = async () => {
    setRefreshing(true);
    await loadTest();
    setRefreshing(false);
  };

  const handleDelete = async () => {
    if (window.confirm('Are you sure you want to delete this test?')) {
      try {
        await deleteTest(id);
        navigate('/');
      } catch (err) {
        alert(errorMessage(err, 'Failed to delete test'));
        console.error(err);
      }
    }
  };

  const handleReRun = async () => {
    try {
      const result = await rerunTest(id);
      if (result && result.id) {
        navigate(`/tests/${result.id}`);
      } else {
        // Fallback if ID is not returned for some reason
        navigate('/');
      }
    } catch (err) {
      alert(errorMessage(err, 'Failed to re-run test'));
      console.error(err);
    }
  };

  const handleStop = async () => {
    if (window.confirm('Are you sure you want to stop this test?')) {
      try {
        await stopTest(id);
        await loadTest();
      } catch (err) {
        alert(errorMessage(err, 'Failed to stop test'));
        console.error(err);
      }
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <Loader2 className="animate-spin text-blue-600" size={48} />
      </div>
    );
  }

  if (!test) {
    return (
      <div className="space-y-4">
        <Link to="/" className="inline-flex items-center gap-2 text-blue-600 hover:underline">
          <ArrowLeft size={16} /> Back to tests
        </Link>
        <div className="bg-red-50 border border-red-200 text-red-700 p-4 rounded-md">
          {error || 'Test not found'}
        </div>
      </div>
    );
  }

  const status = testStatus(test);
  const envVars = Object.entries(test.envVars || {});
  const secretEnvVars = Object.entries(test.secretEnvVars || {});

  return (
    <div className="space-y-8">
      <div className="flex flex-col md:flex-row justify-between gap-4">
        <div className="space-y-2">
          <Link to="/" className="inline-flex items-center gap-2 text-slate-500 hover:text-blue-600 transition-colors mb-2">
            <ArrowLeft size={16} /> Back to tests
          </Link>
          <h1 className="text-3xl font-bold text-slate-900 flex items-center gap-3">
            {test.name}
            {TITLE_ICONS[status.category]}
          </h1>
          <p className="text-slate-500 font-mono text-sm">{test.id}</p>
        </div>
        <div className="flex items-start gap-3">
          <button
            onClick={handleRefresh}
            className="btn btn-outline gap-2"
            disabled={refreshing}
            title="Refresh"
          >
            <RefreshCcw size={16} className={refreshing ? 'animate-spin' : ''} />
          </button>
          {!test.finished && (
            <button
              onClick={handleStop}
              className="btn bg-orange-100 text-orange-700 hover:bg-orange-200 border-orange-200 gap-2"
            >
              <Square size={16} />
              Stop Test
            </button>
          )}
          <button
            onClick={handleReRun}
            className="btn btn-primary gap-2"
          >
            <Play size={16} />
            Re-Run
          </button>
          <button
            onClick={handleDelete}
            className="btn btn-danger gap-2"
          >
            <Trash2 size={16} />
            Delete Test
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 p-4 rounded-md">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="card p-6 space-y-4">
          <h3 className="text-sm font-semibold text-slate-500 uppercase tracking-wider flex items-center gap-2">
            <Activity size={16} /> Status Info
          </h3>
          <div className="space-y-3">
            <Row label="Status"><StatusBadge test={test} /></Row>
            <Row label="Stage">{test.phase}</Row>
            <Row label="Namespace">{test.namespace}</Row>
            {status.detail && <p className="text-xs text-slate-500">{status.detail}</p>}
          </div>
        </div>

        <div className="card p-6 space-y-4">
          <h3 className="text-sm font-semibold text-slate-500 uppercase tracking-wider flex items-center gap-2">
            <Calendar size={16} /> Timing
          </h3>
          <div className="space-y-3">
            <Row label="Started">{formatTime(test.startedAt)}</Row>
            {test.stoppedAt && <Row label="Stopped">{formatTime(test.stoppedAt)}</Row>}
            <Row label="Finished">{formatTime(test.finishedAt)}</Row>
          </div>
        </div>

        <div className="card p-6 space-y-4">
          <h3 className="text-sm font-semibold text-slate-500 uppercase tracking-wider flex items-center gap-2">
            <Server size={16} /> Resources
          </h3>
          <div className="space-y-3">
            <Row label="Parallelism"><span className="font-semibold">{test.parallelism}</span></Row>
            <Row label="Runner Image"><span className="text-xs font-mono">{test.runnerImage || '-'}</span></Row>
            <Row label="ConfigMap">
              <span className="text-xs font-mono bg-slate-100 px-1 rounded">{test.configMap}</span>
            </Row>
          </div>
        </div>
      </div>

      <div className="card p-6 space-y-4">
        <h3 className="text-sm font-semibold text-slate-500 uppercase tracking-wider flex items-center gap-2">
          <Settings size={16} /> Configuration
        </h3>
        <div className="space-y-3">
          <Row label="k6 Arguments">
            <span className="text-xs font-mono">{test.args || '-'}</span>
          </Row>
          <div className="space-y-2">
            <span className="text-slate-600">Environment Variables</span>
            {envVars.length === 0 && secretEnvVars.length === 0 ? (
              <p className="text-sm text-slate-500 italic">None</p>
            ) : (
              <table className="w-full text-sm font-mono">
                <tbody className="divide-y divide-slate-100">
                  {envVars.map(([name, value]) => (
                    <tr key={name}>
                      <td className="py-1 pr-4 text-slate-900 align-top">{name}</td>
                      <td className="py-1 text-slate-600 break-all">{value}</td>
                    </tr>
                  ))}
                  {secretEnvVars.map(([name, ref]) => (
                    <tr key={name}>
                      <td className="py-1 pr-4 text-slate-900 align-top">{name}</td>
                      <td className="py-1 text-slate-600 break-all">
                        <span className="inline-flex items-center gap-1" title="Read from a Kubernetes Secret">
                          <KeyRound size={12} /> secret {ref.name} / {ref.key}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </div>

      <div className="space-y-4">
        <h2 className="text-xl font-bold text-slate-900 flex items-center gap-2">
          <FileCode size={20} /> k6 Script
        </h2>
        <div className="card overflow-hidden border-slate-300">
          <div className="bg-slate-50 px-4 py-2 border-b border-slate-200 flex justify-between items-center">
            <span className="text-sm font-mono text-slate-600">{test.scriptFile || 'script.js'}</span>
          </div>
          <CodeEditor
            height="400px"
            value={test.script || '// Script content not available'}
            options={{
              readOnly: true,
              minimap: { enabled: false },
              fontSize: 14,
              scrollBeyondLastLine: false,
              automaticLayout: true,
            }}
          />
        </div>
      </div>
    </div>
  );
};

// Key the page by test ID so navigating to another test (e.g. after a re-run)
// starts from a clean loading state instead of showing the previous test.
const TestDetail = () => {
  const { id } = useParams();
  return <TestDetailPage key={id} id={id} />;
};

export default TestDetail;
