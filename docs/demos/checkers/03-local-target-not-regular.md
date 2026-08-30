---
title: 引用目录而不是文件
description: 演示检查器如何拒绝指向目录的本地文件引用。
tags:
  - 文档检查
  - 本地引用
create_date: 2026-08-30
update_date: 2026-08-30
checker: local-target.not-regular
---

# 引用目录而不是文件

检查规则：`local-target.not-regular`

下面这个链接指向 `assets/` 目录——路径存在，但它不是普通文件，浏览器无法
把它当作一张图或一页文档打开：

[资源目录不是普通文件](assets)

预期表现：

- `m2h check docs/demos/checkers/03-local-target-not-regular.md` 报告
  `19:2: error [local-target.not-regular]: target "assets" is not a regular file`

[返回检查规则演示索引](00-index.md)
