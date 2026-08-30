---
title: 同一章节下重复标题
---

# 部署手册

## 安装

两个"安装"是同一个父章节（部署手册）的直接子标题，锚点 ID 会被动加
`-1` 后缀：

## 安装

预期表现：

- `m2h check docs/demos/checkers/16-heading-duplicate.md` 报告
  `12:1: warning [heading.duplicate]: duplicate heading "安装" in the same section`
- 不同父章节下的同名标题（例如各章节都有自己的"用法"）完全合法；
  重复 H1 只由 `document.multiple-h1` 报告，不重复计数
