---
title: 环境变量 (.env) 语法高亮示例
description: .env.example 使用 bash 围栏标签，Chroma 将环境变量文件按 Shell 语法高亮。
tags:
  - 语法高亮
  - 配置文件
create_date: 2026-09-06
update_date: 2026-09-06
---

# 环境变量 (.env) 语法高亮示例

.env.example 使用 bash 围栏标签，Chroma 将环境变量文件按 Shell 语法高亮。

```bash
# .env 示例文件 - 环境变量配置，KEY=VALUE 形式，# 为注释
# 用法：复制为 .env 并填入实际值；命令行 export 或 docker --env-file 加载

# ================ 应用环境 ================
APP_ENV=development          # development / staging / production
APP_PORT=8080
APP_NAME="My App"
DEBUG=true

# ================ 数据库 ================
DB_HOST=127.0.0.1
DB_PORT=5432
DB_NAME=myapp
DB_USER=postgres
DB_PASSWORD=secret

# ================ 带引号的值 ================
# 含空格或特殊字符时用引号；单双引号均可
APP_TITLE="Hello World"
CITY='北京'

# ================ API 密钥 ================
API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
JWT_SECRET=your-secret-key-here

# ================ 多行值说明 ================
# .env 通常不支持原生多行；可用 \n 转义（由应用解析）
# 或将证书等转为 base64 单行存储
CERT_BASE64=LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t

# ================ 布尔（由应用自行解析）================
ENABLE_CACHE=1
DISABLE_TELEMETRY=false

# ================ 列表（由应用自行拆分）================
CORS_ORIGINS=https://a.com,https://b.com
```
