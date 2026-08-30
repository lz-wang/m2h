# 恶意 Raw HTML 回归文档

这份文档用于验证内容安全策略对内联事件脚本的拦截：Markdown 的 raw HTML 按输入原样渲染，`onerror` 内联事件处理器在 `script-src 'self'` 下不允许执行，因此 `window.__m2h_xss` 必须始终未定义。图片本身指向一个不存在的路径，加载失败会触发 `error` 事件——没有 CSP 时这正是 XSS 的执行路径。

<img src="/missing-csp-probe.png" onerror="window.__m2h_xss = true" alt="内联事件探针">

<script>window.__m2h_xss_inline_script = true</script>

正文结尾。上面的两个负载都没有执行时，本文档在任何监听地址下都是安全的。
