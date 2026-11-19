---
title: Agent 知识库接入
description: 使用长期 API Token 通过搜索与原始 Markdown 接口将 m2h 文档库接入 LLM Agent 的三步规范。
tags:
  - agent
  - api
create_date: 2026-11-18
update_date: 2026-11-20
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
- 标注的 Markdown 和 JSON 复制按钮会生成绑定当前文档版本的 30 分钟临时 Token，并将授权信息放入剪贴板。文档修改后读取返回 `409 Conflict`，请重新复制。
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
- 临时 Token 最长有效 24 小时。长期 Token 可设置有效期或选择永久有效。撤销立即生效；明文只在创建时展示一次。
- `POST /api/tokens` 的长期 Token 可使用 `permanent: true` 创建永久凭证，不能同时提供 `expiresInDays`；响应中 `expiresAt: 0` 表示永久有效。默认有效期仍为 180 天。
- 临时 Token 仅支持单篇文档的 `document:read` 权限；旧的超范围临时 Token 不再可用。令牌列表只显示未过期且未撤销的 Token；过期超过 7 天的临时 Token 和撤销超过 7 天的 Token 会定期清理。
- 反向代理部署下请使用对外地址；`Authorization` 头不要放进 URL 查询参数。


## 按文档日期浏览

`GET /api/recent` 返回 `created` 和 `updated` 两组结果，各最多 10 条。每条包含 `path`、`title`、`date` 和 `dateSource`。多根工作区的 `path` 带 `r0/` 等根前缀。认证模式下，API 令牌须有 `document:read` 权限，结果仅包含其范围内可读取的文档。

日期为 `YYYY-MM-DD`。有效 Frontmatter 日期的 `dateSource` 为 `frontmatter`。使用文件修改日期时为 `mtime`，此时创建日期为估算值。同一天的条目按文件修改时间倒序排列，再按路径升序排列。

单篇文档令牌直接读取该文档的元数据。绑定文档版本的令牌仅在内容哈希匹配时返回条目。其他请求使用共享元数据缓存，其有效期为 30 秒。新增和编辑的文档可能要等缓存刷新后才出现。缓存通过文件路径、目标路径、大小和修改时间判定文件是否变化。原地改写时若保留大小和修改时间，缓存可能继续显示旧元数据。
