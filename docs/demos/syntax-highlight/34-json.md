---
title: JSON 语法高亮示例
description: 以 json 围栏代码块渲染 data.json，验证 JSON 的语法高亮效果。
tags:
  - 语法高亮
  - 配置文件
create_date: 2026-09-06
update_date: 2026-09-06
---

# JSON 语法高亮示例

以下 `data.json` 示例使用 ` ```json ` 围栏代码块渲染：

```json
{
  "_comment": "JSON 示例文件 - 轻量数据交换格式，语法严格，不支持注释与尾逗号",
  "_features": "演示对象/数组/嵌套/各基础类型，用 _comment 字段代替注释",

  "appName": "demo-service",
  "version": "1.2.0",
  "releaseDate": "2025-03-15",
  "stable": true,
  "deprecated": false,
  "downloads": null,
  "rating": 4.7,
  "userCount": 102400,

  "tags": ["api", "web", "demo"],

  "author": {
    "name": "张三",
    "email": "zhangsan@example.com",
    "verified": true,
    "social": null
  },

  "settings": {
    "features": {
      "darkMode": true,
      "betaAccess": false,
      "maxConnections": 10
    },
    "limits": [100, 200, 300],
    "nested": {
      "deep": {
        "value": "多层级嵌套"
      }
    }
  },

  "environments": [
    { "name": "dev",  "url": "http://localhost:8080",  "debug": true  },
    { "name": "prod", "url": "https://api.example.com", "debug": false }
  ],

  "unicodeExample": "中文 / 日本語 / 한국어 / emoji 🚀",
  "escapedString": "引号 \" 转义 / 反斜杠 \\ / 换行符 \\n / 制表 \\t",
  "emptyObject": {},
  "emptyArray": []
}
```
