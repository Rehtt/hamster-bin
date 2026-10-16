import assert from 'node:assert/strict';
import test from 'node:test';
import React from 'react';
import { renderToString } from 'react-dom/server';
import { MemoryRouter } from 'react-router-dom';
import { createServer } from 'vite';

// 复用 Vite 的 TSX 加载器与 React 服务端渲染，无需浏览器或额外测试依赖。
const server = await createServer({
  server: { middlewareMode: true, ws: false, hmr: false, watch: null },
  optimizeDeps: { noDiscovery: true, include: [] },
  logLevel: 'error',
});
try {
  const { AuthContext } = await server.ssrLoadModule('/src/context/auth.ts');
  const { default: ProtectedRoute } = await server.ssrLoadModule('/src/components/ProtectedRoute.tsx');
  const { default: Login } = await server.ssrLoadModule('/src/pages/Login.tsx');
  const defaults = {
    authEnabled: true,
    passwordEnabled: false,
    oauthEnabled: false,
    isAuthenticated: false,
    username: null,
    loading: false,
    error: null,
    login: async () => {},
    logout: async () => {},
    refresh: async () => {},
  };
  function render(Component, state = {}, path = '/login') {
    return renderToString(React.createElement(
      AuthContext.Provider, { value: { ...defaults, ...state } },
      React.createElement(MemoryRouter, { initialEntries: [path] },
        React.createElement(Component, null, React.createElement('p', null, 'private-inventory'))),
    ));
  }
  for (const mode of [
    { name: 'password', passwordEnabled: true, oauthEnabled: false },
    { name: 'oauth', passwordEnabled: false, oauthEnabled: true },
    { name: 'both', passwordEnabled: true, oauthEnabled: true },
  ]) {
    await test(`login methods: ${mode.name}`, () => {
      const html = render(Login, mode);
      assert.equal(html.includes('<form'), mode.passwordEnabled);
      assert.equal(html.includes('使用统一账号登录'), mode.oauthEnabled);
    });
  }
  await test('authentication failure blocks inventory and offers retry', () => {
    for (const Component of [ProtectedRoute, Login]) {
      const html = render(Component, { error: '无法获取登录状态，请检查网络后重试' });
      assert.ok(html.includes('role="alert"'));
      assert.ok(html.includes('重试'));
      assert.ok(!html.includes('private-inventory'));
      assert.ok(!html.includes('<form'));
    }
  });
  await test('loading and unauthenticated states block inventory', () => {
    assert.ok(!render(ProtectedRoute, { loading: true }).includes('private-inventory'));
    assert.ok(!render(ProtectedRoute).includes('private-inventory'));
  });
  await test('authenticated and authentication-disabled states show inventory', () => {
    assert.ok(render(ProtectedRoute, { isAuthenticated: true }).includes('private-inventory'));
    assert.ok(render(ProtectedRoute, { authEnabled: false, isAuthenticated: true }).includes('private-inventory'));
    assert.ok(!render(Login, { isAuthenticated: true }).includes('<form'));
    assert.ok(!render(Login, { authEnabled: false }).includes('<form'));
  });
  for (const [code, message] of [
    ['access_denied', '没有应用访问权'],
    ['unavailable', '统一登录暂不可用'],
    ['invalid', '请重新登录'],
    ['untrusted-provider-message', '请重新登录'],
  ]) {
    await test(`OAuth callback error: ${code}`, () => {
      const html = render(Login, { oauthEnabled: true }, `/login?oauth_error=${code}`);
      assert.ok(html.includes(message));
      assert.ok(!html.includes('untrusted-provider-message'));
    });
  }
} finally {
  await server.close();
}
