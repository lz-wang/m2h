---
title: TOML 语法高亮示例
description: 以 toml 围栏代码块渲染 config.toml，验证 TOML 的语法高亮效果。
tags:
  - 语法高亮
  - 配置文件
create_date: 2026-09-06
update_date: 2026-09-06
---

# TOML 语法高亮示例

以下 `config.toml` 示例使用 ` ```toml ` 围栏代码块渲染：

```toml
# TOML 示例文件 - 语义明确的配置格式，类 ini 但类型丰富、支持嵌套表

# ================ 顶层键值对（根表）================
title = "TOML 示例"
owner = "张三"
enabled = true

# ================ 各基础类型 ================
name = "demo"                          # 字符串
port = 8080                            # 整数
pi = 3.14                              # 浮点
flag = true                            # 布尔

# 多行字符串（字面，三个单引号）
description = '''
TOML 多行字面字符串，
反斜杠不转义，原样保留。
'''

# 日期时间
created = 2025-01-01T08:00:00Z         # 带时区的日期时间
birthday = 1970-01-01                  # 仅日期

# 数组
ports = [8000, 8001, 8002]
tags = ["api", "web"]

# ================ 表 [section] ================
[server]
host = "0.0.0.0"
port = 80

# 嵌套表 [a.b]
[server.tls]
enabled = false
cert = ""

# 内联表
client = { name = "app", version = "1.0" }

# ================ 数组表 [[products]] ================
# 每个 [[products]] 是 products 数组中的一个表元素
[[products]]
name = "Hammer"
sku = 738594937
price = 12.50

[[products]]
name = "Nail"
sku = 284758393
price = 0.50

# ================ 数据库配置 ================
[database]
url = "postgres://localhost/db"
pool = 10
nodes = ["n1", "n2", "n3"]
```
