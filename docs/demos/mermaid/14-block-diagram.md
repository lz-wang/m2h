---
title: 块图 (Block Diagram)
description: Mermaid 块图语法参考
tags:
  - mermaid
  - block-diagram
create_date: 2026-08-29
update_date: 2026-08-29
---

块图用块和连接器直观地表示复杂系统。与流程图不同，块图让作者完全控制形状的定位。

```mermaid
block
columns 1
  db(("DB"))
  blockArrowId6<["&nbsp;&nbsp;&nbsp;"]>(down)
  block:ID
    A
    B["A wide one in the middle"]
    C
  end
  space
  D
  ID --> D
  C --> D
  style B fill:#969,stroke:#333,stroke-width:4px
```

````markdown
```mermaid
block
columns 1
  db(("DB"))
  blockArrowId6<["&nbsp;&nbsp;&nbsp;"]>(down)
  block:ID
    A
    B["A wide one in the middle"]
    C
  end
  space
  D
  ID --> D
  C --> D
  style B fill:#969,stroke:#333,stroke-width:4px
```
````

## 基本结构

```mermaid
block
  a b c
```

````markdown
```mermaid
block
  a b c
```
````

### 设置列数

```mermaid
block
  columns 3
  a b c d
```

````markdown
```mermaid
block
  columns 3
  a b c d
```
````

## 高级配置

### 设置块宽度

用 `:N` 让块跨越 N 列：

```mermaid
block
  columns 3
  a["A label"] b:2 c:2 d
```

````markdown
```mermaid
block
  columns 3
  a["A label"] b:2 c:2 d
```
````

### 组合块（嵌套）

```mermaid
block
    block
      D
    end
    A["A: I am a wide one"]
```

````markdown
```mermaid
block
    block
      D
    end
    A["A: I am a wide one"]
```
````

### 动态列宽

列宽由该列中最宽的块自动决定。

```mermaid
block
  columns 3
  a:3
  block:group1:2
    columns 2
    h i j k
  end
  g
  block:group2:3
    l m n o p q r
  end
```

````markdown
```mermaid
block
  columns 3
  a:3
  block:group1:2
    columns 2
    h i j k
  end
  g
  block:group2:3
    l m n o p q r
  end
```
````

## 块形状

| 形状 | 语法 |
|---|---|
| 圆角 | `id1("text")` |
| 体育场形 | `id1(["text"])` |
| 子程序 | `id1([["text"]])` |
| 圆柱（数据库） | `id1[("Database")]` |
| 圆形 | `id1(("text"))` |
| 菱形 | `id1{"text"}` |
| 六边形 | `id1{{"text"}}` |
| 平行四边形 | `id1[/"text"/]` |
| 双圆 | `id1((("text")))` |

### 块箭头

```mermaid
block
  blockArrowId<["Label"]>(right)
  blockArrowId2<["Label"]>(left)
  blockArrowId3<["Label"]>(up)
  blockArrowId4<["Label"]>(down)
```

````markdown
```mermaid
block
  blockArrowId<["Label"]>(right)
  blockArrowId2<["Label"]>(left)
  blockArrowId3<["Label"]>(up)
  blockArrowId4<["Label"]>(down)
```
````

### 空间块

用 `space` 创建空白区域，可用 `space:N` 指定占据 N 列：

```mermaid
block
  columns 3
  a space b
```

````markdown
```mermaid
block
  columns 3
  a space b
```
````

## 连接块

```mermaid
block
  A space B
  A-->B
```

````markdown
```mermaid
block
  A space B
  A-->B
```
````

### 带文本的链接

```mermaid
block
  A space:2 B
  A-- "X" -->B
```

````markdown
```mermaid
block
  A space:2 B
  A-- "X" -->B
```
````

## 样式

### 直接样式

```text
style id1 fill:#636,stroke:#333,stroke-width:4px
```

### 类样式

```mermaid
block
  A space B
  A-->B
  classDef blue fill:#6e6ce6,stroke:#333,stroke-width:4px
  class A blue
```

````markdown
```mermaid
block
  A space B
  A-->B
  classDef blue fill:#6e6ce6,stroke:#333,stroke-width:4px
  class A blue
```
````

## 实用示例

### 系统架构

```mermaid
block
  columns 3
  Frontend blockArrowId6<[" "]>(right) Backend
  space:2 down<[" "]>(down)
  Disk left<[" "]>(left) Database[("Database")]

  classDef front fill:#696,stroke:#333
  classDef back fill:#969,stroke:#333
  class Frontend front
  class Backend,Database back
```

````markdown
```mermaid
block
  columns 3
  Frontend blockArrowId6<[" "]>(right) Backend
  space:2 down<[" "]>(down)
  Disk left<[" "]>(left) Database[("Database")]

  classDef front fill:#696,stroke:#333
  classDef back fill:#969,stroke:#333
  class Frontend front
  class Backend,Database back
```
````

## 参考

- [Block Diagram - Mermaid](https://mermaid.js.org/syntax/block.html)
