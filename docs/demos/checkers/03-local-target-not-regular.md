---
title: 引用目录而不是文件
---

# 引用目录而不是文件

下面这个链接指向 `assets/` 目录——路径存在，但它不是普通文件，浏览器无法
把它当作一张图或一页文档打开：

[资源目录不是普通文件](assets)

预期表现：

- `m2h check docs/demos/checkers/03-local-target-not-regular.md` 报告
  `10:2: error [local-target.not-regular]: target "assets" is not a regular file`
