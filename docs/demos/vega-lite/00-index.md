---
title: Vega-Lite 统计图表演示索引
description: m2h 的 Vega-Lite 围栏统计图表用法、能力边界与兼容矩阵，含条形、折线、散点、热力、直方与交互图示例入口。
tags:
  - vega-lite
  - 图表
create_date: 2026-09-02
update_date: 2026-09-02
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
| 远程 `data.url` | 不支持 |
| 本地 `data.url` | 暂不支持 |
| YAML spec | 不支持 |
| 原生 Vega spec | 不支持 |
| Vega-Embed 操作菜单 | 关闭（宿主策略） |

## 安全与自包含契约

图表 spec 必须自包含：数据写在 `data.values` 中，运行时拒绝一切外部资源加载（`data.url`、字符串 `config`、字符串 `patch`），错误信息为 `external Vega-Lite data loading is not supported`。这个契约在 Web 文档服务与导出 HTML 中一致成立——它由宿主提供的加载器保证，而不是依赖某个页面的 CSP，因此导出的 HTML 同样不会发出任何外部数据请求。

文档不能覆盖宿主渲染策略：spec 中 `usermeta.embedOptions` 会被剥离，`mode`（固定 `vega-lite`）、`renderer`（固定 SVG）与 Vega-Embed 自带的导出/编辑器菜单（关闭）由 m2h 决定。

## 主题与 Lightbox

图表外观 chrome（背景、坐标轴、图例与标题的文字和网格颜色）跟随阅读器主题，取值来自页面主题的 CSS 变量；作者在 spec 中定义的数据颜色（mark color、scale.range）不会被宿主覆盖，切换主题不会改变图表的数据语义。Web 文档服务中图表支持 Lightbox 放大查看，与图片、Mermaid 图表按文档顺序共用同一查看器。

## 失败隔离

无效 JSON、非对象 spec 或编译失败的图表保留原始源码文本并在控制台输出警告，不影响同一文档中的其他图表与 Mermaid、公式等富内容。
