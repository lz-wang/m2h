---
title: CSS 语法高亮示例
description: 以 css 围栏代码块渲染 css.css，验证 CSS 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# CSS 语法高亮示例

以下 `css.css` 示例使用 ` ```css ` 围栏代码块渲染：

```css
/* CSS3 — 选择器、盒模型、布局、动画与响应式速览 */

/* ============================================================
 * 一、自定义属性（CSS 变量）
 * ============================================================ */

:root {
  --brand: #3b82f6;          /* 主色 */
  --text: #1f2937;           /* 正文色 */
  --bg: #f9fafb;             /* 背景色 */
  --radius: 8px;             /* 圆角 */
  --gap: 16px;               /* 间距 */
}

/* ============================================================
 * 二、选择器：元素 / 类 / 后代 / 子代 / 相邻 / 伪类 / 伪元素
 * ============================================================ */

/* 元素选择器 */
body {
  margin: 0;
  font-family: system-ui, -apple-system, sans-serif;
  color: var(--text);          /* 使用自定义属性 */
  background: var(--bg);
  line-height: 1.6;
}

/* 类选择器 */
.card {
  padding: var(--gap);
  border: 1px solid #e5e7eb;
  border-radius: var(--radius);
}

/* 后代选择器（空格）：.card 内任意层级的 p */
.card p {
  margin: 0 0 8px;
}

/* 子代选择器（>）：仅直接子元素 */
.list > li {
  list-style: square;
}

/* 相邻兄弟选择器（+）：紧随其后的同级元素 */
h2 + p {
  color: #6b7280;
}

/* 伪类：状态与位置 */
a:hover { color: var(--brand); }            /* 悬停 */
a:focus-visible { outline: 2px solid var(--brand); }
.list li:nth-child(odd) { background: #f3f4f6; } /* 奇数行 */
.list li:first-child { font-weight: bold; }
.list li:last-child { border-bottom: none; }

/* 伪元素 ::before / ::after */
.quote::before { content: '\201C'; }   /* 左引号 */
.quote::after  { content: '\201D'; }   /* 右引号 */

/* 通用兄弟选择器（~）与属性选择器 */
input[type="checkbox"]:checked ~ label {
  color: var(--brand);
}

/* ============================================================
 * 三、盒模型与 box-sizing
 * ============================================================ */

* {
  /* border-box：宽度包含 padding 与 border，更直观 */
  box-sizing: border-box;
}

.box {
  width: 200px;
  margin: 10px auto;       /* 外边距，上下 10、左右居中 */
  padding: 12px;           /* 内边距 */
  border: 2px solid #ccc;  /* 边框：粗细 样式 颜色 */
}

/* ============================================================
 * 四、颜色与单位
 * ============================================================ */

.units {
  font-size: 1rem;        /* rem：相对根字号 */
  padding: 1em;           /* em：相对当前元素字号 */
  width: 50%;             /* 百分比：相对父容器 */
  height: 10vw;           /* vw：视口宽度的 1% */
  color: rgb(31, 41, 55);               /* rgb */
  background: hsl(220, 80%, 55%);       /* hsl */
  border-color: #3b82f680;              /* 8 位十六进制，末两位为透明度 */
}

/* 文本样式 */
.text {
  text-align: center;        /* 对齐 */
  text-decoration: underline;/* 装饰线 */
  text-transform: uppercase; /* 大写 */
  letter-spacing: 0.05em;    /* 字间距 */
  font-weight: 600;
  white-space: nowrap;       /* 不换行 */
}

/* 背景 */
.hero {
  background-color: #111827;
  background-image: linear-gradient(135deg, #3b82f6, #8b5cf6);
  background-size: cover;
  background-position: center;
  color: #fff;
}

/* ============================================================
 * 五、布局：Flexbox 简要
 * ============================================================ */

.row {
  display: flex;
  gap: var(--gap);
  align-items: center;     /* 交叉轴居中 */
  justify-content: space-between; /* 主轴两端对齐 */
  flex-wrap: wrap;         /* 允许换行 */
}

.row .item {
  flex: 1 1 200px;         /* 增长 收缩 基准 */
}

/* ============================================================
 * 六、布局：Grid 简要
 * ============================================================ */

.grid {
  display: grid;
  /* 三列等宽，最小列宽 160px，自动填充 */
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: var(--gap);
}

/* ============================================================
 * 七、过渡 transition
 * ============================================================ */

.btn {
  background: var(--brand);
  color: #fff;
  padding: 8px 16px;
  border: none;
  border-radius: var(--radius);
  cursor: pointer;
  /* 过渡：监听属性 时长 缓动 延迟 */
  transition: transform 0.2s ease, background 0.2s ease;
}

.btn:hover {
  transform: translateY(-2px);   /* 向上轻移 */
  background: #2563eb;
}

/* ============================================================
 * 八、动画 @keyframes
 * ============================================================ */

@keyframes pulse {
  0%   { opacity: 1; transform: scale(1); }
  50%  { opacity: 0.5; transform: scale(1.1); }
  100% { opacity: 1; transform: scale(1); }
}

.pulse {
  animation: pulse 1.5s ease-in-out infinite;
}

/* ============================================================
 * 九、响应式 @media
 * ============================================================ */

/* 屏幕宽度 ≤ 768px 时生效 */
@media (max-width: 768px) {
  .row { flex-direction: column; align-items: stretch; }
  .grid { grid-template-columns: 1fr; }
  body { font-size: 0.95rem; }
}

/* ============================================================
 * 十、优先级说明
 * ============================================================
 * 特异性（从高到低）：
 *   1. !important（慎用，覆盖一切）
 *   2. 内联 style="..."               (1000)
 *   3. ID  #id                        (100)
 *   4. 类 / 伪类 / 属性 .class:hover  (10)
 *   5. 元素 / 伪元素 div::before      (1)
 *   6. 通配 * 与组合符                 (0)
 * 同等特异性时，后写的规则胜出（源顺序）。
 * ============================================================ */
```
