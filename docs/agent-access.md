---
title: Agent 知识库接入
description: 使用长期 API Token 通过搜索与原始 Markdown 接口将 m2h 文档库接入 LLM Agent 的三步规范。
tags:
  - agent
  - api
create_date: 2026-11-18
update_date: 2026-11-18
---

# Agent 知识库接入

认证模式（`--auth local` 或 `--auth ldap`）下的 m2h 提供两步式只读知识库接口：先全文搜索定位文档，再取原始 Markdown。凭据为 API Token（在 WebUI 的「API 令牌」设置中创建，或在标注面板一键生成临时分享）。默认无认证模式下这些接口同样工作，只是不需要 `Authorization` 头。

## 1. 搜索相关文档

```bash
curl -fsS \
  -H "Authorization: Bearer $M2H_TOKEN" \
  --get \
  --data-urlencode "q=LDAP认证" \
  "https://docs.example.com/api/search"
```

- 关键词检索（非语义检索）；中文按字符分词，英文不区分大小写。
- 单次最多返回 50 条，按相关性排序；结果包含路径、标题、摘要和可选的标题锚点（`heading`）。
- 索引有约 30 秒 TTL 的增量刷新：刚修改的文档可能短暂搜不到，读取原文时始终以当前文件为准。
- Token 的范围（全部文档 / 单个根 / 单篇文档）在检索过程中生效——越范围的文档不会出现在结果里。

## 2. 获取 Markdown 原文

```bash
curl -fsS \
  -H "Authorization: Bearer $M2H_TOKEN" \
  "https://docs.example.com/raw/design.md"
```

- 原文即源文件字节（含 Frontmatter）；文档路径与搜索结果的 `path` 一致，多根工作区带 `r0/`、`r1/` 前缀。
- 标注面板「复制给 Agent」生成的临时 Token 绑定文档当前版本：文档被修改后读取返回 `409 Conflict`，请重新向文档所有者索取。
- Token 不能读取文档附件（图片等）；如需要，请让所有者单独提供。

## 3. 基于检索结果回答并提供来源

回答时保留文档路径、标题与标题锚点（拼成 `https://docs.example.com/doc/<path>#<heading.id>` 即可让人类回到 m2h 原文位置），不要只给结论。

对 Agent 侧 Skill 的工具语义建议：

```
search_documents(query)   → GET /api/search?q=…
read_document(path)       → GET /raw/<path>
list_documents()          → GET /api/files
```

这只是 Agent 侧的接口抽象；m2h 后端保持以上 REST API 不变。

## 边界与限制

- 第一版不提供 MCP Server、Embedding、向量数据库或独立 RAG 服务。
- Token 权限为只读（`document:read`、`search:read`），不能写入标注、不能管理 Token、不能调用登录接口。
- Token 有过期时间（临时最长 24 小时，长期最长 365 天），撤销立即失效；明文只在创建时展示一次。
- 临时 Token 仅支持单篇文档的 `document:read` 权限；旧的超范围临时 Token 不再可用。令牌列表只显示未过期且未撤销的 Token；过期超过 7 天的临时 Token 和撤销超过 7 天的 Token 会定期清理。
- 反向代理部署下请使用对外地址；`Authorization` 头不要放进 URL 查询参数。
