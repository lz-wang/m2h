---
title: 非法 UTF-8 字节
description: 说明 document.invalid-utf8 的判定与复现步骤。
tags:
  - 文档检查
  - 编码
create_date: 2026-11-14
update_date: 2026-11-14
checker: document.invalid-utf8
---

# 非法 UTF-8 字节

检查规则：`document.invalid-utf8`

本演示文件本身完全干净：包含非法字节的文件不能进入仓库，因此这里只
说明规则语义并给出复现步骤。

规则在解析 Frontmatter 或 Markdown 之前检查整个文件的原始字节。任何
非法 UTF-8 序列（孤立的 `0xFF`、被截断的多字节序列等）都会让后续所有
阶段——YAML、Goldmark、WebUI——把作者的文字解码成替换字符，因此检测到
非法字节时只报告 UTF-8 错误与结尾空白行检查，不再产生衍生诊断；同一
文件多处非法字节只报告第一处。UTF-8 BOM 与合法的 `U+FFFD` 替换字符
本身不算错误。

复现步骤（在临时目录执行，不要把产物提交进仓库）：

```bash
printf '# Guide\n\nbroken \xff here\n' > broken-utf8.md
m2h check broken-utf8.md
```

预期表现：

- 上面的命令报告
  `broken-utf8.md:3:8: error [document.invalid-utf8]: document contains an invalid UTF-8 byte sequence (0xFF)`
- 行列从 1 开始，列按源码字节偏移计算；CRLF 文件同样正确定位
- Go 单元测试（`internal/check/rule_source_test.go`）在临时目录生成
  非法字节文件覆盖这些场景

[返回检查规则演示索引](00-index.md)
