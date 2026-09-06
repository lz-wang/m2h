---
title: HTML 语法高亮示例
description: 以 html 围栏代码块渲染 html.html，验证 HTML 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# HTML 语法高亮示例

以下 `html.html` 示例使用 ` ```html ` 围栏代码块渲染：

```html
<!DOCTYPE html>
<!-- HTML5 — 语义化文档骨架示例，浏览器可直接打开 -->
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta name="description" content="HTML5 语法速览示例页面">
  <title>HTML5 示例页面</title>
</head>
<body>

  <!-- ============ 页眉 / 导航 ============ -->
  <header>
    <h1>HTML5 语法速览</h1>
    <nav aria-label="主导航">
      <a href="#semantic">语义化</a> |
      <a href="#list">列表</a> |
      <a href="#table">表格</a> |
      <a href="#form">表单</a>
    </nav>
  </header>

  <!-- ============ 主体内容 ============ -->
  <main>
    <!-- 语义化区块：section + article -->
    <section id="semantic">
      <h2>语义化标签</h2>
      <article>
        <h3>什么是语义化？</h3>
        <p>
          使用有意义的标签（如 <code>&lt;article&gt;</code>、<code>&lt;aside&gt;</code>）
          表达内容结构，便于无障碍阅读和 SEO。
          <em>强调</em>、<strong>重要</strong>、<mark>标记</mark> 是常见的行内语义。
        </p>
        <blockquote cite="https://html.spec.whatwg.org/">
          语义化让机器也能读懂页面结构。
        </blockquote>
      </article>
    </section>

    <!-- 链接与图片（alt 是无障碍必需） -->
    <section>
      <h2>链接与图片</h2>
      <p>
        <a href="https://developer.mozilla.org/" target="_blank" rel="noopener">
          MDN Web 文档
        </a>
      </p>
      <p>
        <img
          src="https://placehold.co/320x180"
          alt="占位图：320x180 示例"
          width="320" height="180">
      </p>
    </section>

    <!-- ============ 列表：ul / ol / dl ============ -->
    <section id="list">
      <h2>列表</h2>
      <h3>无序列表 ul</h3>
      <ul>
        <li>苹果</li>
        <li>香蕉</li>
      </ul>

      <h3>有序列表 ol</h3>
      <ol start="1">
        <li>第一步：打开编辑器</li>
        <li>第二步：编写 HTML</li>
      </ol>

      <h3>描述列表 dl</h3>
      <dl>
        <dt>HTML</dt>
        <dd>超文本标记语言</dd>
        <dt>CSS</dt>
        <dd>层叠样式表</dd>
      </dl>
    </section>

    <!-- ============ 表格 ============ -->
    <section id="table">
      <h2>表格</h2>
      <table border="1" cellpadding="6">
        <thead>
          <tr>
            <th scope="col">姓名</th>
            <th scope="col">年龄</th>
            <th scope="col">城市</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>张三</td>
            <td>28</td>
            <td>北京</td>
          </tr>
          <tr>
            <td>李四</td>
            <td>34</td>
            <td>上海</td>
          </tr>
        </tbody>
      </table>
    </section>

    <!-- ============ 表单 ============ -->
    <section id="form">
      <h2>表单</h2>
      <form action="/submit" method="post">
        <p>
          <label for="name">姓名：</label>
          <input type="text" id="name" name="name" placeholder="请输入姓名" required>
        </p>
        <p>
          <label for="email">邮箱：</label>
          <input type="email" id="email" name="email" required>
        </p>
        <p>
          <label for="pwd">密码：</label>
          <input type="password" id="pwd" name="pwd" minlength="6">
        </p>
        <p>
          <label for="age">年龄：</label>
          <input type="number" id="age" name="age" min="0" max="150" value="18">
        </p>
        <p>
          <label for="bio">简介：</label>
          <textarea id="bio" name="bio" rows="3" cols="30"></textarea>
        </p>
        <p>
          <label for="city">城市：</label>
          <select id="city" name="city">
            <option value="bj">北京</option>
            <option value="sh" selected>上海</option>
            <option value="gz">广州</option>
          </select>
        </p>
        <fieldset>
          <legend>订阅</legend>
          <label><input type="checkbox" name="news" checked> 新闻</label>
          <label><input type="checkbox" name="promo"> 促销</label>
        </fieldset>
        <p>
          <label><input type="radio" name="plan" value="free" checked> 免费</label>
          <label><input type="radio" name="plan" value="pro"> 专业</label>
        </p>
        <p>
          <label for="vol">音量：</label>
          <input type="range" id="vol" name="vol" min="0" max="100">
          <input type="date" name="birthday">
        </p>
        <button type="submit">提交</button>
        <button type="reset">重置</button>
      </form>
    </section>

    <!-- 内嵌 SVG 简要 -->
    <section>
      <h2>内嵌 SVG</h2>
      <svg width="120" height="60" viewBox="0 0 120 60" xmlns="http://www.w3.org/2000/svg">
        <rect x="5" y="5" width="110" height="50" rx="8"
              fill="#3b82f6" stroke="#1e3a8a" stroke-width="2"/>
        <text x="60" y="35" text-anchor="middle" fill="#fff" font-size="14">SVG</text>
      </svg>
    </section>
  </main>

  <!-- ============ 侧边栏（补充内容） ============ -->
  <aside>
    <h2>相关链接</h2>
    <ul>
      <li><a href="https://html.spec.whatwg.org/">HTML 标准</a></li>
      <li><a href="https://www.w3.org/WAI/">无障碍 WAI</a></li>
    </ul>
  </aside>

  <!-- ============ 页脚 ============ -->
  <footer>
    <hr>
    <p><small>&copy; 2026 HTML5 示例 · 仅作语法演示</small></p>
  </footer>

</body>
</html>
```
