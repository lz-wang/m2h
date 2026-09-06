---
title: 韦恩图 (Venn Diagram)
description: Mermaid 韦恩图语法参考
tags:
  - mermaid
  - venn
create_date: 2026-08-29
update_date: 2026-08-29
---

韦恩图使用重叠圆形展示集合之间的关系。



> [!WARNING]
> 这是 Mermaid 的新图表类型，语法可能在后续版本中演进。
>

```mermaid
venn-beta
  title "Team overlap"
  set Frontend
  set Backend
  union Frontend,Backend["APIs"]
```

````markdown
```mermaid
venn-beta
  title "Team overlap"
  set Frontend
  set Backend
  union Frontend,Backend["APIs"]
```
````

## 语法

- 以 `venn-beta` 开始
- `set` 定义集合
- `union` 定义两个或多个集合的重叠
- `union` 中的标识符必须由之前的 `set` 定义
- 标识符可以是裸词或引号字符串

### 标签

用 `["..."]` 设置显示标签：

```markdown
set A["Alpha"]
set B["Beta"]
union A,B["AB"]
```

### 大小

用 `:N` 后缀设置大小：

```markdown
set A["Alpha"]:20
set B["Beta"]:12
union A,B["AB"]:3
```

### 文本节点

- `text` 在集合或联合内放置标签
- 缩进的 `text` 附加到最近的 `set` 或 `union`

```mermaid
venn-beta
  set A["Frontend"]
    text A1["React"]
    text A2["Design Systems"]
  set B["Backend"]
    text B1["API"]
  union A,B["Shared"]
    text AB1["OpenAPI"]
```

````markdown
```mermaid
venn-beta
  set A["Frontend"]
    text A1["React"]
    text A2["Design Systems"]
  set B["Backend"]
    text B1["API"]
  union A,B["Shared"]
    text AB1["OpenAPI"]
```
````

### 样式

使用 `style` 语句设置视觉效果：

- `fill` — 填充色
- `color` — 文本色
- `stroke` — 描边色
- `stroke-width` — 描边宽度
- `fill-opacity` — 填充透明度

```mermaid
venn-beta
  set A["Alpha"]:20
    text A1["React"]
  set B["Beta"]:12
  union A,B["AB"]:3
  style A fill:#ff6b6b
  style A,B color:#333
  style A1 color:red
```

````markdown
```mermaid
venn-beta
  set A["Alpha"]:20
    text A1["React"]
  set B["Beta"]:12
  union A,B["AB"]:3
  style A fill:#ff6b6b
  style A,B color:#333
  style A1 color:red
```
````

## 参考

- [Venn - Mermaid](https://mermaid.js.org/syntax/venn.html)
