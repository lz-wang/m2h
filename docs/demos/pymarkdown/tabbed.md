---
title: PyMdown Tabbed 标签内容
description: 演示标签分组、默认选择、标题导航、搜索与 HTML 导出
tags:
  - markdown
  - 标签
create_date: 2026-10-14
update_date: 2026-10-14
---

# PyMdown Tabbed 标签内容

Tabbed 默认启用，无需 CLI 配置；WebUI、HTML 导出、TOC、全文搜索与 `m2h check`
使用同一 Markdown 解析结果。基础切换使用原生单选控件，无 JavaScript、无 CDN 也可使用。

## 基础语法

- `=== "标题"` 声明标签；连续标签组成一组。
- `===! "标题"` 强制开始另一组。
- `===+ "标题"` 指定默认标签，同组多个默认标记取第一个。
- 标记允许 0–3 个前导空格；正文至少比标记行多缩进 4 列（Tab 按 4 列制表位计算）。
- 标题为双引号包围的非空纯文本（不能全部为空格、Tab 或其他空白字符），不支持内部双引号、Markdown 格式或自定义 ID；特殊 HTML 字符会转义。
- 空标签和单个标签均可使用；格式错误或正文起始缩进不足时保留普通 Markdown 解析。
- 第一版只识别文档顶层标签；列表、引用和标签内部的 Tabbed 标记按普通 Markdown 解析。
- 每组最多 32 个交互标签，超限时展开整组全部正文；不支持 Linked Tabs、嵌套标签、状态持久化与跨文档同步。

## 两组独立标签

第一组默认选中 Go，第二组默认选中 Linux。使用 Tab 聚焦单选组，再以方向键切换，
选中的标签有下边框，键盘焦点有描边；桌面端悬浮未选中标签时文字变为强调色。
手机标签栏横向滚动，不换行。

=== "Python"

    ### 安装

    ```python
    print("Hello from Python")
    ```

    - 第一项
    - 第二项

===+ "Go"

    ### 安装

    ```go
    fmt.Println("Hello from Go")
    ```

    [跳到 Python 安装](#安装)

===! "Linux"

    apt install nginx

=== "macOS"

    brew install nginx tabbed-macos-token

    | 名称 | 数量 |
    | --- | ---: |
    | beta | 2 |
    | alpha | 1 |

    ![示例图片](assets/tabbed.svg)

    ```mermaid
    graph LR
      Source --> Parser --> Tabs
    ```

    行内公式：$x^2+y^2=z^2$。

## 导航、搜索与富内容

两个“安装”标题分别生成 `#安装` 与 `#安装-1`。点击 TOC 或正文链接、
直接打开深链接及刷新都会先显示目标标签；`m2h check` 仍会按现有规则提醒重复标题。
手动切换标签后，滚动跟踪跳过隐藏标题，并清理指向隐藏标题的当前锚点。

在 WebUI 使用 Ctrl/Cmd+K 搜索 `tabbed-macos-token`，会自动激活第二组的 macOS，
定位无内部标题的内容面板。同文档定位不重新加载正文；跨文档定位在文档加载后应用。
搜索 API 的结果可携带可选 `tabId` 字符串，与 HTML 面板的 `data-m2h-tab-id` 一致；
普通文档和仅元数据命中省略该字段。

隐藏标签中的图表、公式和表格正常增强；图片在标签可见且接近视口后加载。
WebUI 支持图片与 Mermaid Lightbox、代码复制和折叠；切换主题不会重置标签选择。
HTML 导出沿用既有公式、图表、表格按需加载策略，打印时展开所有标签并显示各自标题。

## 空标签与特殊字符

===! "空内容 & <示例>"

## 非规范标记

下面保留为普通正文；代码中的标记不会创建标签。

=== 缺少双引号标题

```text
=== "代码中的标记"
=== ""
=== "标题" 多余内容
```

## 验证命令

从仓库根目录浏览和导出：

```console
m2h docs
m2h export docs/demos/pymarkdown/tabbed.md
m2h check docs/demos/pymarkdown/tabbed.md
```

单文件检查只会提示本例刻意重复的“安装”标题，不影响默认成功退出。
从目录范围单独检查本演示及引用时使用：

```console
m2h check docs --depth 8 --glob 'demos/pymarkdown/tabbed.md'
```
