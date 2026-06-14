import React, { useState, useEffect } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { getTest, deleteTest, rerunTest, stopTest } from '../api/client';
import { 
  ArrowLeft, 
  Trash2, 
  Clock, 
  Layers, 
  FileCode, 
  Server,
  Calendar,
  Activity,
  CheckCircle2,
  AlertCircle,
  Loader2,
  ExternalLink,
  Play,
  Square
} from 'lucide-react';
import { format } from 'date-fns';
import Editor from '@monaco-editor/react';

const TestDetail = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const [test, setTest] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchTest = async () => {
      try {
        const data = await getTest(id);
        setTest(data);
      } catch (err) {
        setError('Failed to fetch test details');
        console.error(err);
      } finally {
        setLoading(false);
      }
    };
    fetchTest();
  }, [id]);

  const handleDelete = async () => {
    if (window.confirm('Are you sure you want to delete this test?')) {
      try {
        await deleteTest(id);
        navigate('/');
      } catch (err) {
        alert('Failed to delete test');
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
      alert('Failed to re-run test');
      console.error(err);
    }
  };

  const handleStop = async () => {
    if (window.confirm('Are you sure you want to stop this test?')) {
      try {
        await stopTest(id);
        // Refresh test details to show updated phase (though it might take a moment to reflect in k8s)
        const data = await getTest(id);
        setTest(data);
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

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <Loader2 className="animate-spin text-blue-600" size={48} />
      </div>
    );
  }

  if (error || !test) {
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

  return (
    <div className="space-y-8">
      <div className="flex flex-col md:flex-row justify-between gap-4">
        <div className="space-y-2">
          <Link to="/" className="inline-flex items-center gap-2 text-slate-500 hover:text-blue-600 transition-colors mb-2">
            <ArrowLeft size={16} /> Back to tests
          </Link>
          <h1 className="text-3xl font-bold text-slate-900 flex items-center gap-3">
            {test.name}
            {isFinished(test.phase) && (test.phase?.toLowerCase() === 'succeeded' || test.phase?.toLowerCase() === 'completed' || test.phase?.toLowerCase() === 'finished') && <CheckCircle2 className="text-green-500" size={28} />}
            {isFinished(test.phase) && (test.phase?.toLowerCase() === 'failed' || test.phase?.toLowerCase() === 'error' || test.phase?.toLowerCase() === 'errored') && <AlertCircle className="text-red-500" size={28} />}
            {!isFinished(test.phase) && <Loader2 className="text-blue-500 animate-spin" size={28} />}
          </h1>
          <p className="text-slate-500 font-mono text-sm">{test.id}</p>
        </div>
        <div className="flex items-start gap-3">
          {!isFinished(test.phase) && (
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

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="card p-6 space-y-4">
          <h3 className="text-sm font-semibold text-slate-500 uppercase tracking-wider flex items-center gap-2">
            <Activity size={16} /> Status Info
          </h3>
          <div className="space-y-3">
            <div className="flex justify-between">
              <span className="text-slate-600">Phase</span>
              <span className={`font-semibold ${
                (test.phase?.toLowerCase() === 'succeeded' || test.phase?.toLowerCase() === 'completed' || test.phase?.toLowerCase() === 'finished') ? 'text-green-600' : 
                (test.phase?.toLowerCase() === 'failed' || test.phase?.toLowerCase() === 'error' || test.phase?.toLowerCase() === 'errored') ? 'text-red-600' : 
                !isFinished(test.phase) ? 'text-blue-600' : 'text-slate-900'
              }`}>{test.phase}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-600">Namespace</span>
              <span className="text-slate-900">{test.namespace}</span>
            </div>
          </div>
        </div>

        <div className="card p-6 space-y-4">
          <h3 className="text-sm font-semibold text-slate-500 uppercase tracking-wider flex items-center gap-2">
            <Calendar size={16} /> Timing
          </h3>
          <div className="space-y-3">
            <div className="flex justify-between">
              <span className="text-slate-600">Started</span>
              <span className="text-slate-900">{test.startedAt ? format(new Date(test.startedAt), 'MMM d, HH:mm:ss') : '-'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-600">Finished</span>
              <span className="text-slate-900">{test.finishedAt ? format(new Date(test.finishedAt), 'MMM d, HH:mm:ss') : '-'}</span>
            </div>
          </div>
        </div>

        <div className="card p-6 space-y-4">
          <h3 className="text-sm font-semibold text-slate-500 uppercase tracking-wider flex items-center gap-2">
            <Server size={16} /> Resources
          </h3>
          <div className="space-y-3">
            <div className="flex justify-between">
              <span className="text-slate-600">Parallelism</span>
              <span className="text-slate-900 font-semibold">{test.parallelism}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-600">ConfigMap</span>
              <span className="text-slate-900 text-xs font-mono bg-slate-100 px-1 rounded">{test.configMap}</span>
            </div>
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
          <Editor
            height="400px"
            defaultLanguage="javascript"
            value={test.script || '// Script content not available in summary'}
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

export default TestDetail;
