import axios from 'axios';

const client = axios.create({
  baseURL: '/api',
});

export const getTests = async () => {
  const response = await client.get('/tests');
  return response.data;
};

export const getTest = async (id) => {
  const response = await client.get(`/tests/${id}`);
  return response.data;
};

export const createTest = async (testData) => {
  // Use FormData because we might have a file upload
  const formData = new FormData();
  formData.append('name', testData.name);
  formData.append('parallelism', testData.parallelism);
  formData.append('runnerImage', testData.runnerImage || 'grafana/k6:latest');
  
  if (testData.scriptFile) {
    formData.append('script', testData.scriptFile);
  } else if (testData.scriptContent) {
    const blob = new Blob([testData.scriptContent], { type: 'application/javascript' });
    formData.append('script', blob, 'script.js');
  }

  if (testData.envVars) {
    Object.entries(testData.envVars).forEach(([key, value]) => {
      formData.append(`envVars[${key}]`, value);
    });
  }

  if (testData.args) {
    formData.append('args', testData.args);
  }

  const response = await client.post('/tests', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  });
  return response.data;
};

export const deleteTest = async (id) => {
  const response = await client.delete(`/tests/${id}`);
  return response.data;
};

export default client;
