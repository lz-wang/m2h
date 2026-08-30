---
title: 疑似错误编码文本
---

# 疑似错误编码文本

下面这行文字是"咖啡馆"的 UTF-8 字节被按 Latin-1 误解码后又重新编码的
典型产物（正确写法应为"一杯 Café"）：

一杯 CafÃ©，编码出错的拿铁。

引号类签名同理：Itâ€™s 出自弯引号 `’`。

预期表现：

- 默认不检查：`m2h check docs/demos/checkers/24-unicode-mojibake.md` 无诊断
- `--enable unicode.mojibake` 后按签名逐处报告
  `10:11` 与 `12:27: warning [unicode.mojibake]: suspicious mojibake "Ã©"`（另一处为 `"â€™"`）
- 只用多字符签名判定：单独一个 `Ã` 或 `Â` 是多门语言的合法字母，不报
  （行内代码里的内容也永不参与判定）
