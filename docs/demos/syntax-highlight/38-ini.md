---
title: INI 语法高亮示例
description: 以 ini 围栏代码块渲染 app.ini，验证 INI 的语法高亮效果。
tags:
  - 语法高亮
  - 配置文件
create_date: 2026-09-06
update_date: 2026-09-06
---

# INI 语法高亮示例

以下 `app.ini` 示例使用 ` ```ini ` 围栏代码块渲染：

```ini
; INI 示例文件 - 简单的分节配置格式，所有值均为字符串
; 注释以分号 ; 或井号 # 开头

# ================ [database] 数据库节 ================
[database]
host = localhost
port = 5432
name = mydb
user = postgres
password = secret
; 多值约定一：逗号分隔（由应用自行拆分）
nodes = node1,node2,node3
pool_size = 10

# ================ [server] 服务节 ================
[server]
host = 0.0.0.0
port = 8080
debug = true                ; 注意：INI 不区分类型，true 在此只是字符串 "true"
workers = 4
; 多值约定二：重复键（部分解析器支持多值）
allow = read
allow = write
allow = delete

# ================ [logging] 日志节 ================
[logging]
level = info
path = /var/log/app.log
rotate = daily

# ================ [features] 功能开关 ================
[features]
enable_cache = 1
enable_metrics = 0
retry = 3
```
