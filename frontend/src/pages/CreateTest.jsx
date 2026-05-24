import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { createTest } from '../api/client';
import { 
  ArrowLeft, 
  Save, 
  Upload, 
  Code, 
  Plus, 
  X,
  Info,
  Loader2
} from 'lucide-react';
import Editor from '@monaco-editor/react';

const CreateTest = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [creationMethod, setCreationMethod] = useState('editor'); // 'editor' or 'upload'
  
  const [formData, setFormData] = useState({
    name: '',
    parallelism: 1,
    runnerImage: '',
    args: '',
  });
  
  const [scriptContent, setScriptContent] = useState(`import http from 'k6/http';
import { sleep } from 'k6';

export const options = {
  vus: 10,
  duration: '30s',
};

export default function () {
  http.get('https://test.k6.io');
  sleep(1);
}`);
  
  const [scriptFile, setScriptFile] = useState(null);
  const [envVars, setEnvVars] = useState([{ key: '', value: '' }]);

  const handleInputChange = (e) => {
    const { name, value } = e.target;
    setFormData(prev => ({ ...prev, [name]: value }));
  };

  const handleEnvVarChange = (index, field, value) => {
    const newEnvVars = [...envVars];
    newEnvVars[index][field] = value;
    setEnvVars(newEnvVars);
  };

  const addEnvVar = () => {
    setEnvVars([...envVars, { key: '', value: '' }]);
  };

  const removeEnvVar = (index) => {
    setEnvVars(envVars.filter((_, i) => i !== index));
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      const envVarsMap = {};
      envVars.forEach(ev => {
        if (ev.key) envVarsMap[ev.key] = ev.value;
      });

      const submissionData = {
        ...formData,
        envVars: envVarsMap,
        scriptContent: creationMethod === 'editor' ? scriptContent : null,
        scriptFile: creationMethod === 'upload' ? scriptFile : null,
      };

      await createTest(submissionData);
      navigate('/');
    } catch (err) {
      console.error(err);
      let errorMsg = 'Failed to create test. Please check your inputs.';
      if (err.response?.data) {
        const data = err.response.data;
        if (data.detail) {
          errorMsg = data.detail;
        } else if (data.errors) {
          errorMsg = Object.entries(data.errors)
            .map(([field, msgs]) => `${field}: ${msgs.join(', ')}`)
            .join(' | ');
        }
      }
      setError(errorMsg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-4xl mx-auto space-y-8">
      <div className="space-y-2">
        <Link to="/" className="inline-flex items-center gap-2 text-slate-500 hover:text-blue-600 transition-colors mb-2">
          <ArrowLeft size={16} /> Back to tests
        </Link>
        <h1 className="text-3xl font-bold text-slate-900">Create New Test</h1>
        <p className="text-slate-500">Configure and launch a new k6 load test</p>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        {error && (
          <div className="bg-red-50 border border-red-200 text-red-700 p-4 rounded-md flex gap-3">
            <AlertCircle className="shrink-0" />
            <p>{error}</p>
          </div>
        )}

        <div className="card p-6 space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="space-y-2">
              <label className="text-sm font-semibold text-slate-700" htmlFor="name">Test Name *</label>
              <input 
                id="name"
                name="name"
                type="text" 
                required
                placeholder="e.g. Homepage Load Test"
                className="input"
                value={formData.name}
                onChange={handleInputChange}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-semibold text-slate-700" htmlFor="parallelism">Parallelism (Instances) *</label>
              <input 
                id="parallelism"
                name="parallelism"
                type="number" 
                min="1"
                required
                className="input"
                value={formData.parallelism}
                onChange={handleInputChange}
              />
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="space-y-2">
              <label className="text-sm font-semibold text-slate-700" htmlFor="runnerImage">Runner Image</label>
              <input 
                id="runnerImage"
                name="runnerImage"
                type="text" 
                className="input"
                value={formData.runnerImage}
                onChange={handleInputChange}
              />
              <p className="text-xs text-slate-400">Docker image to use for k6 runners</p>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-semibold text-slate-700" htmlFor="args">Additional k6 Args</label>
              <input 
                id="args"
                name="args"
                type="text" 
                placeholder="e.g. --tag env=prod"
                className="input"
                value={formData.args}
                onChange={handleInputChange}
              />
            </div>
          </div>
        </div>

        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <h2 className="text-xl font-bold text-slate-900 flex items-center gap-2">
              <Code size={20} /> k6 Script
            </h2>
            <div className="flex bg-slate-100 p-1 rounded-md">
              <button 
                type="button"
                className={`px-3 py-1 text-sm font-medium rounded ${creationMethod === 'editor' ? 'bg-white shadow-sm text-blue-600' : 'text-slate-600'}`}
                onClick={() => setCreationMethod('editor')}
              >
                Editor
              </button>
              <button 
                type="button"
                className={`px-3 py-1 text-sm font-medium rounded ${creationMethod === 'upload' ? 'bg-white shadow-sm text-blue-600' : 'text-slate-600'}`}
                onClick={() => setCreationMethod('upload')}
              >
                Upload File
              </button>
            </div>
          </div>

          <div className="card overflow-hidden border-slate-300">
            {creationMethod === 'editor' ? (
              <Editor
                height="400px"
                defaultLanguage="javascript"
                value={scriptContent}
                onChange={(value) => setScriptContent(value)}
                options={{
                  minimap: { enabled: false },
                  fontSize: 14,
                  automaticLayout: true,
                }}
              />
            ) : (
              <div className="p-12 flex flex-col items-center justify-center border-2 border-dashed border-slate-300 rounded-lg m-4">
                <Upload size={48} className="text-slate-300 mb-4" />
                <input 
                  type="file" 
                  accept=".js"
                  className="hidden" 
                  id="file-upload"
                  onChange={(e) => setScriptFile(e.target.files[0])}
                />
                <label htmlFor="file-upload" className="btn btn-outline cursor-pointer mb-2">
                  Choose File
                </label>
                <p className="text-sm text-slate-500">
                  {scriptFile ? `Selected: ${scriptFile.name}` : 'Upload your k6 script (.js)'}
                </p>
              </div>
            )}
          </div>
        </div>

        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <h2 className="text-xl font-bold text-slate-900">Environment Variables</h2>
            <button type="button" onClick={addEnvVar} className="btn btn-outline btn-sm gap-1">
              <Plus size={14} /> Add Variable
            </button>
          </div>
          <div className="space-y-3">
            {envVars.map((ev, index) => (
              <div key={index} className="flex gap-3 items-start">
                <input 
                  placeholder="KEY" 
                  className="input flex-1"
                  value={ev.key}
                  onChange={(e) => handleEnvVarChange(index, 'key', e.target.value)}
                />
                <input 
                  placeholder="VALUE" 
                  className="input flex-1"
                  value={ev.value}
                  onChange={(e) => handleEnvVarChange(index, 'value', e.target.value)}
                />
                <button 
                  type="button" 
                  onClick={() => removeEnvVar(index)}
                  className="p-2 text-slate-400 hover:text-red-600 transition-colors mt-1"
                >
                  <X size={20} />
                </button>
              </div>
            ))}
            {envVars.length === 0 && (
              <p className="text-slate-500 text-sm italic">No environment variables added.</p>
            )}
          </div>
        </div>

        <div className="pt-6 border-t border-slate-200 flex justify-end gap-4">
          <Link to="/" className="btn btn-outline">Cancel</Link>
          <button 
            type="submit" 
            className="btn btn-primary min-w-[150px] gap-2"
            disabled={loading}
          >
            {loading ? <Loader2 size={18} className="animate-spin" /> : <Save size={18} />}
            Create & Launch
          </button>
        </div>
      </form>
    </div>
  );
};

export default CreateTest;
