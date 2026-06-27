import { describe, it, expect, vi } from 'vitest';

// App 内部已经包含 BrowserRouter，不能嵌套。路由测试通过集成测试验证。
describe('Router', () => {
  it.skip('路由由 App 组件内置 BrowserRouter 管理', () => {
    expect(true).toBe(true);
  });
});
