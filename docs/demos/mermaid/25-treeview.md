---
title: 树视图 (TreeView)
description: Mermaid 树视图语法参考
tags:
  - mermaid
  - treeview
create_date: 2026-08-29
update_date: 2026-08-29
---

树视图用于以目录结构形式表示分层数据，支持文件类型图标、连接线和可选注解。

```mermaid
treeView-beta
    my-project/
        src/
            index.js
        package.json
        README.md
```

````markdown
```mermaid
treeView-beta
    my-project/
        src/
            index.js
        package.json
        README.md
```
````

## 语法

树结构仅通过缩进定义。标签可以是裸词或引号字符串。

- 目录以 `/` 结尾，显示文件夹图标和粗体文本
- 文件按扩展名自动检测并分配匹配图标
- 引号标签支持名称中的空格

## 制表符输入

替代缩进，可使用制表符字符定义树结构，支持标准（`├──`、`└──`、`│`）和粗体（`┣━━`、`┗━━`、`┃`）Unicode 变体：

```mermaid
treeView-beta
├── src/
│   ├── index.ts
│   └── utils.ts
├── package.json
└── README.md
```

````markdown
```mermaid
treeView-beta
├── src/
│   ├── index.ts
│   └── utils.ts
├── package.json
└── README.md
```
````

深度根据分支字符的列位置推断。

## 注解

### 高亮 `:::class`

```mermaid
treeView-beta
    src/
        App.tsx :::highlight
        index.js
    package.json
```

````markdown
```mermaid
treeView-beta
    src/
        App.tsx :::highlight
        index.js
    package.json
```
````

### 行内描述 `##`

`##` 后的文本以斜体显示在标签旁：

```mermaid
treeView-beta
    src/
        index.js ## app entry point
        config.ts ## runtime configuration
    package.json ## project manifest
```

````markdown
```mermaid
treeView-beta
    src/
        index.js ## app entry point
        config.ts ## runtime configuration
    package.json ## project manifest
```
````

### 图标覆盖 `icon()`

```mermaid
treeView-beta
    data/
        model.bin icon(database)
        weights.h5 icon(database)
    src/
        index.js
```

````markdown
```mermaid
treeView-beta
    data/
        model.bin icon(database)
        weights.h5 icon(database)
    src/
        index.js
```
````

### 组合注解

注解可按任意顺序组合：

```mermaid
treeView-beta
    my-project/
        src/
            App.tsx :::highlight icon(react) ## main component
            index.js ## entry point
        .env ## environment variables
        Dockerfile
        package.json
```

````markdown
```mermaid
treeView-beta
    my-project/
        src/
            App.tsx :::highlight icon(react) ## main component
            index.js ## entry point
        .env ## environment variables
        Dockerfile
        package.json
```
````

## 注释

用 `%%` 添加不可见的注释。

## 配置

| 属性 | 说明 | 默认值 |
|---|---|---|
| rowIndent | 每行缩进 | 10 |
| paddingX | 行水平内边距 | 5 |
| paddingY | 行垂直内边距 | 5 |
| lineThickness | 线条粗细 | 1 |
| showIcons | 是否显示文件/文件夹图标 | true |

### 主题变量

| 属性 | 说明 | 默认值 |
|---|---|---|
| labelFontSize | 标签字号 | 16px |
| labelColor | 标签颜色 | black |
| lineColor | 线条颜色 | black |
| iconColor | 图标颜色 | #546e7a |
| descriptionColor | 描述文本色 | #6a9955 |
| highlightBg | 高亮背景 | rgba(255,193,7,0.15) |

## 支持的图标

| 扩展名/文件名 | 图标 |
|---|---|
| `.js`, `.mjs`, `.cjs` | javascript |
| `.ts` | typescript |
| `.jsx`, `.tsx` | react |
| `.py` | python |
| `.json` | json |
| `.md`, `.mdx` | markdown |
| `.html`, `.htm` | html |
| `.css`, `.scss` | css |
| `.yaml`, `.yml` | yaml |
| `.sh`, `.bash` | terminal |
| `.sql`, `.db` | database |
| `Dockerfile` | docker |
| 目录（`/`） | folder |

## 参考

- [TreeView - Mermaid](https://mermaid.js.org/syntax/treeView.html)
