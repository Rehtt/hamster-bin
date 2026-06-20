import axios from 'axios';

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
  withCredentials: true,
});

client.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error.response?.status;
    const url = error.config?.url || '';

    if (
      status === 401 &&
      !url.includes('/auth/me') &&
      !url.includes('/auth/login') &&
      window.location.pathname !== '/login'
    ) {
      window.location.href = '/login';
    }

    return Promise.reject(error);
  }
);

export default client;
