---
title: 看板 (Kanban)
description: Mermaid 看板图语法参考
tags:
  - mermaid
  - kanban
create_date: 2026-08-29
update_date: 2026-08-29
---

看板图用于可视化任务在不同工作流阶段中的移动。

## 基本语法

以 `kanban` 关键字开始，后跟列（阶段）和列内任务的定义。

```mermaid
kanban
  column1[Column Title]
    task1[Task Description]
```

````markdown
```mermaid
kanban
  column1[Column Title]
    task1[Task Description]
```
````

## 定义列

列代表工作流的不同阶段：

```text
columnId[Column Title]
```

## 添加任务

任务缩进在对应列下方：

```text
taskId[Task Description]
```

## 任务元数据

使用 `@{ ... }` 语法添加元数据：

- `assigned` — 负责人
- `ticket` — 关联的工单号
- `priority` — 优先级（'Very High'、'High'、'Low'、'Very Low'）

```mermaid
kanban
todo[Todo]
  id3[Update Database Function]@{ ticket: MC-2037, assigned: 'knsv', priority: 'High' }
```

````markdown
```mermaid
kanban
todo[Todo]
  id3[Update Database Function]@{ ticket: MC-2037, assigned: 'knsv', priority: 'High' }
```
````

## 配置

```yaml
---
config:
  kanban:
    ticketBaseUrl: 'https://yourproject.atlassian.net/browse/#TICKET#'
---
```

`ticketBaseUrl` 设置外部系统的工单链接，`#TICKET#` 会被替换为实际的工单号。

## 完整示例

```mermaid
---
config:
  kanban:
    ticketBaseUrl: 'https://mermaidchart.atlassian.net/browse/#TICKET#'
---
kanban
  Todo
    [Create Documentation]
    docs[Create Blog about the new diagram]
  [In progress]
    id6[Create renderer so that it works in all cases.]
  id9[Ready for deploy]
    id8[Design grammar]@{ assigned: 'knsv' }
  id10[Ready for test]
    id4[Create parsing tests]@{ ticket: MC-2038, assigned: 'K.Sveidqvist', priority: 'High' }
  id11[Done]
    id5[define getData]
    id3[Update DB function]@{ ticket: MC-2037, assigned: knsv, priority: 'High' }
```

````markdown
```mermaid
---
config:
  kanban:
    ticketBaseUrl: 'https://mermaidchart.atlassian.net/browse/#TICKET#'
---
kanban
  Todo
    [Create Documentation]
    docs[Create Blog about the new diagram]
  [In progress]
    id6[Create renderer so that it works in all cases.]
  id9[Ready for deploy]
    id8[Design grammar]@{ assigned: 'knsv' }
  id10[Ready for test]
    id4[Create parsing tests]@{ ticket: MC-2038, assigned: 'K.Sveidqvist', priority: 'High' }
  id11[Done]
    id5[define getData]
    id3[Update DB function]@{ ticket: MC-2037, assigned: knsv, priority: 'High' }
```
````

## 参考

- [Kanban - Mermaid](https://mermaid.js.org/syntax/kanban.html)
