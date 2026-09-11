---
title: 正文图片居中回归文档
description: 验证普通图片、链接、picture、SVG 和图表在正文中的居中布局，以及 GFM Alert 与居中规则共存。
tags:
  - 测试
  - 图片
create_date: 2026-09-10
update_date: 2026-09-11
---

# 正文图片居中回归文档

![普通图片](images/tiny.png)

[![链接图片](images/tiny.png)](#正文图片居中回归文档)

![SVG 图片](images/architecture.svg)

<picture><source srcset="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAIAAACQkWg2AAAAFklEQVR4nGMI6r5EEmIY1TCqYfhqAAChra8QqdkevQAAAABJRU5ErkJggg==" type="image/png"><img src="images/landscape.png" alt="picture 图片" width="80" height="80"></picture>

前文 <img src="images/tiny.png" alt="混排图片"> 后文。

![同段第一张](images/tiny.png) ![同段第二张](images/tiny.png)

<svg xmlns="http://www.w3.org/2000/svg" width="120" height="60" viewBox="0 0 120 60" role="img" aria-label="原生 SVG"><rect width="120" height="60" fill="steelblue"/></svg>

<div><svg xmlns="http://www.w3.org/2000/svg" width="120" height="60" viewBox="0 0 120 60" role="img" aria-label="容器内 SVG"><rect width="120" height="60" fill="seagreen"/></svg></div>

```mermaid
flowchart LR
  A[图片] --> B[居中]
```

```vega-lite
{
  "width": 160,
  "height": 100,
  "data": {"values": [{"x": "甲", "y": 2}, {"x": "乙", "y": 3}]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "x", "type": "nominal"},
    "y": {"field": "y", "type": "quantitative"}
  }
}
```

> [!NOTE]
> Alert layout regression.
