import axios from 'axios';

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
});

client.interceptors.response.use(
  (response) => response,
  (error) => {
    return Promise.reject(error);
  }
);

export default client;
