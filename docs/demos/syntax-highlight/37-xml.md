---
title: XML 语法高亮示例
description: 以 xml 围栏代码块渲染 data.xml，验证 XML 的语法高亮效果。
tags:
  - 语法高亮
  - 配置文件
create_date: 2026-09-06
update_date: 2026-09-06
---

# XML 语法高亮示例

以下 `data.xml` 示例使用 ` ```xml ` 围栏代码块渲染：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!-- XML 示例文件 - 可扩展标记语言，用标签描述结构化数据 -->
<catalog xmlns="https://example.com/ns/books"
         xmlns:dc="http://purl.org/dc/elements/1.1/">

  <!-- ====== 带属性与命名空间前缀的元素 ====== -->
  <book id="b1" available="true">
    <dc:title>XML 入门</dc:title>          <!-- 命名空间前缀 dc -->
    <author>张三</author>
    <price currency="CNY">59.00</price>

    <!-- 实体引用：< > & " ' 需转义 -->
    <symbols>比较：a &lt; b &amp;&amp; c &gt; d</symbols>
    <quote>他说：&quot;你好&quot;</quote>

    <!-- CDATA：内容不被解析，原样保留（含 < > & 等）-->
    <code><![CDATA[
      if (a < b && c > d) {
        console.log("< > & 原样输出");
      }
    ]]></code>

    <!-- 自闭合元素 -->
    <cover src="cover.jpg" alt="封面"/>

    <!-- 嵌套列表 -->
    <tags>
      <tag>xml</tag>
      <tag>教程</tag>
    </tags>
  </book>

  <!-- ====== 第二本书 ====== -->
  <book id="b2" available="false">
    <dc:title>进阶指南</dc:title>
    <author>李四</author>
    <price currency="USD">29.99</price>
    <tags>
      <tag>advanced</tag>
    </tags>
  </book>

</catalog>
```
