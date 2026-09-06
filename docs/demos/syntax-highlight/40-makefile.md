---
title: Makefile 语法高亮示例
description: 以 makefile 围栏代码块渲染 Makefile，验证 Makefile 的语法高亮效果。
tags:
  - 语法高亮
  - 配置文件
create_date: 2026-09-06
update_date: 2026-09-06
---

# Makefile 语法高亮示例

以下 `Makefile` 示例使用 ` ```makefile ` 围栏代码块渲染：

```makefile
# Makefile 示例文件 - GNU Make 构建自动化，命令行必须用 Tab 缩进（非空格）

# ================ 变量赋值方式 ================
CC = gcc                  # 递归展开（延迟，使用时才求值）
CFLAGS ?= -O2             # 条件赋值（仅未定义时才赋值）
TARGET := app             # 立即赋值（定义时展开一次）
SRCS += main.c utils.c    # 追加赋值（在原值后追加）

# ================ 条件判断 ================
ifdef DEBUG
CFLAGS += -g -DDEBUG
else
CFLAGS += -DNDEBUG
endif

# ================ 内置函数 ================
OBJS = $(patsubst %.c,%.o,$(SRCS))          # patsubst：模式替换
FILES = $(wildcard src/*.c)                  # wildcard：通配符展开文件列表

# ================ 默认目标（make 不带参数时执行）================
all: $(TARGET)

# 链接目标：依赖 + 命令（以下命令行均以 Tab 开头）
$(TARGET): $(OBJS)
	$(CC) $(CFLAGS) -o $@ $^
	@echo "构建完成: $@"

# ================ 模式规则（% 匹配任意子串）================
%.o: %.c
	$(CC) $(CFLAGS) -c $< -o $@

# ================ 伪目标（不对应实际文件）================
.PHONY: clean test run install help

clean:
	rm -f $(OBJS) $(TARGET)

test:
	@echo "运行测试..."

run: $(TARGET)
	./$(TARGET)

install: $(TARGET)
	install -m 755 $(TARGET) /usr/local/bin/

help:
	@echo "可用目标: all clean test run install help"
	@echo "自动变量: $@=目标  $<=首个依赖  $^=所有依赖"
```
