---
title: YAML 语法高亮示例
description: 以 yaml 围栏代码块渲染 config.yaml，验证 YAML 的语法高亮效果。
tags:
  - 语法高亮
  - 配置文件
create_date: 2026-09-06
update_date: 2026-09-06
---

# YAML 语法高亮示例

以下 `config.yaml` 示例使用 ` ```yaml ` 围栏代码块渲染：

```yaml
# YAML 示例文件 - 层级数据序列化格式，靠缩进表达结构（空格，非 Tab）
# 本文件演示：标量、列表、映射、多行字符串、锚点与别名、类型推断

# ================ 标量与映射 ================
app:
  name: demo-service        # 字符串标量
  version: "1.2.0"          # 加引号避免被推断为数字
  port: 8080                # 整数
  ratio: 0.95               # 浮点数
  enabled: true             # 布尔
  timeout: null             # null（也可写 ~ 或留空）

# ================ 列表（块序列）================
tags:
  - api
  - web
  - "yes"                   # 加引号避免被推断为布尔 true
  - "123"                   # 加引号保留为字符串

# 流式风格（内联列表）
flags: [a, b, c]

# ================ 多行字符串 ================
description: |
  字面块标量（|）：保留换行，
  适合写代码或保留排版的文本。
  末尾换行保留（| 会保留，|- 去除尾部空行）。

summary: >
  折叠块标量（>）：换行会被折叠成空格，
  适合写连续段落，输出合并为单行
  （段落间空行会保留为换行）。

# ================ 类型推断说明 ================
inference:
  int_val: 42               # -> 整数
  float_val: 3.14           # -> 浮点
  bool_yes: yes             # -> true（注意歧义！）
  bool_quoted: "yes"        # -> 字符串 "yes"
  null_a: null              # -> null
  null_b: ~                 # -> null
  null_c:                   # 空值 -> null
  iso_date: 2025-01-01      # -> 日期（PyYAML 等会解析）

# ================ 锚点与别名 ================
defaults: &def              # &def 定义锚点
  retries: 3
  timeout: 30

service_a:
  <<: *def                  # *def 引用锚点，<< 合并到当前映射
  name: svc-a

service_b:
  <<: *def
  name: svc-b
  timeout: 60               # 覆盖锚点中的同名键

# ================ 嵌套结构 ================
servers:
  - host: node-1
    roles: [web, cache]
  - host: node-2
    roles: [db]
    meta:
      zone: us-east-1
```
