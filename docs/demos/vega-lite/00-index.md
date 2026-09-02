---
title: Vega-Lite 统计图表演示索引
description: m2h 的 Vega-Lite 围栏统计图表用法、能力边界与兼容矩阵，含条形、折线、散点、热力、直方与交互图示例入口。
tags:
  - vega-lite
  - 图表
create_date: 2026-09-02
update_date: 2026-09-03
---

# Vega-Lite 统计图表演示索引

m2h 支持在 Markdown 中嵌入 [Vega-Lite](https://vega.github.io/vega-lite/) 统计图表：把 JSON spec 写进 ` ```vega-lite ` 围栏代码块（别名 ` ```vegalite `），浏览器按需加载渲染运行时并输出 SVG 图表。Web 文档服务从内嵌运行时渲染，导出 HTML 从固定版本的 CDN 加载同一组运行时，两端行为一致。

```bash
m2h docs/demos/vega-lite
```

| 文件 | 演示内容 |
| --- | --- |
| 01-bar-line | 条形图、折线图与分层图；坐标轴、标题与别名 fence |
| 02-scatter | 散点图与颜色编码字段 |
| 03-heatmap | 热力图（`rect` mark + 颜色分级） |
| 04-histogram | 直方图（内置 bin 聚合） |
| 05-interactive | 交互选择与 tooltip |

## 兼容矩阵

| 能力 | 状态 |
| --- | --- |
| `vega-lite` fence | 支持 |
| `vegalite` 别名 | 支持 |
| JSON spec | 支持（必须为 JSON 对象） |
| Vega-Lite v6 | 支持 |
| `data.values` 内联数据 | 支持 |
| transforms / 聚合 | 支持 |
| 分层 / 拼接 / 分面 | 支持 |
| 交互选择与参数 | 支持 |
| tooltip | 支持 |
| 远程 `data.url` | 不支持（嵌入前判定，按失败隔离） |
| 本地 `data.url` | 暂不支持 |
| 外部图片 mark | 不支持（不加载，图表仍渲染） |
| 图表超链接（`href` 通道） | 支持（跨站新标签打开） |
| YAML spec | 不支持 |
| 原生 Vega spec | 不支持 |
| Vega-Embed 操作菜单 | 关闭（宿主策略） |

## 安全与自包含契约

图表 spec 必须自包含：数据写在 `data.values` 中。引用外部数据的 spec（顶层 `data.url`、`datasets` 条目、lookup `from.data`，以及分层/拼接/分面子图表中的同类位置）在嵌入前即被判定为不支持，控制台报 `Vega-Lite specification must be self-contained`，图表按失败隔离处理——保留源码文本、不渲染 SVG、不提供放大按钮。字符串 `config`、字符串 `patch` 与外部图片 URL 则由宿主加载器在加载时拒绝（`external Vega-Lite data loading is not supported`），不会发出任何网络请求。这个契约在 Web 文档服务与导出 HTML 中一致成立——它由宿主策略保证，而不是依赖某个页面的 CSP，因此导出的 HTML 同样不会发出任何外部数据请求。

图表超链接（`href` 编码通道）的点击是导航而不是数据加载，遵循与正文链接相同的阅读器策略：跨 origin 的 HTTP(S) 链接在新标签页打开并带 `noopener noreferrer`（图表链接由 Vega 在点击时动态合成，不经正文链接增强，显式 `noreferrer` 是合理的一致防线；WebUI 另有 `Referrer-Policy: same-origin` 响应头），同源链接保持浏览器默认行为，`javascript:` 等其他协议一律拒绝——spec 不能把图表 mark 变成脚本执行入口。

文档不能覆盖宿主渲染策略：spec 中 `usermeta.embedOptions` 会被剥离，`mode`（固定 `vega-lite`）、`renderer`（固定 SVG）、表达式求值路径（固定 AST 解释器，页面 CSP 无需 `unsafe-eval`）与 Vega-Embed 自带的导出/编辑器菜单（关闭）由 m2h 决定。

## 主题与 Lightbox

图表外观 chrome（背景、坐标轴、图例与标题的文字和网格颜色）跟随阅读器主题，取值来自页面主题的 CSS 变量；作者在 spec 中定义的数据颜色（mark color、scale.range）不会被宿主覆盖，切换主题不会改变图表的数据语义。Web 文档服务中图表支持 Lightbox 放大查看，与图片、Mermaid 图表按文档顺序共用同一查看器。

## 失败隔离

无效 JSON、非对象 spec 或编译失败的图表保留原始源码文本并在控制台输出警告，不影响同一文档中的其他图表与 Mermaid、公式等富内容。
