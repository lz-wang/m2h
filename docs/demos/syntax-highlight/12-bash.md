---
title: Bash 语法高亮示例
description: 以 bash 围栏代码块渲染 bash.sh，验证 Bash 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Bash 语法高亮示例

以下 `bash.sh` 示例使用 ` ```bash ` 围栏代码块渲染：

```bash
#!/usr/bin/env bash
# Bash — Shell 脚本语言，用于系统自动化与命令编排。
# 严格模式：遇错即停、未定义变量报错、管道失败传播
set -euo pipefail

# ============================================================
# 变量与引号（单 / 双 / 命令替换 $()）
# ============================================================
name="World"               # 赋值：等号两侧不能有空格
single='No $name here'     # 单引号：原样输出，不展开
double="Hello, $name!"     # 双引号：变量展开
today=$(date +%F)          # 命令替换 $(...)
echo "$single / $double / $today"

# ============================================================
# 参数扩展（${var:-default} 等）
# ============================================================
default="${undefined:-fallback}"   # 变量为空/未定义时取默认值
keep="${name:-anonymous}"          # 变量已设置则保留
length="${#name}"                  # 字符串长度
substr="${name:0:3}"               # 子串 -> "Wor"
upper="${name^^}"                  # 转大写
replaced="${name/World/Bash}"      # 替换首个匹配
trimmed="${name#W}"                # 去最短前缀 -> "orld"
suffix="${name%ld}"                # 去最短后缀 -> "Wor"
echo "$default | len=$length | $substr | $upper | $replaced | $trimmed | $suffix"

# ============================================================
# 算术运算 $(( ))
# ============================================================
count=5
(( count++ ))                 # 自增（count 起始非 0，set -e 安全）
(( count *= 2 ))              # 乘法赋值
echo "count=$count, 1+1=$(( 1 + 1 ))"

# ============================================================
# 条件判断：test / [[ ]] / case
# ============================================================
if [[ "$name" == "World" ]]; then        # [[ ]] 字符串比较
    echo "string match"
elif [[ -f /etc/hosts ]]; then           # 文件存在判断
    echo "file exists"
fi

[[ -d /tmp ]] && echo "/tmp is a directory"   # 短路 &&

case "$name" in
    World) echo "hello world" ;;
    Bash)  echo "shell mode" ;;
    *)     echo "unknown: $name" ;;
esac

# ============================================================
# 循环：for / while / until
# ============================================================
for i in 1 2 3; do
    echo "loop $i"
done

for (( i = 0; i < 3; i++ )); do           # C 风格 for
    echo "c-style $i"
done

n=0
while (( n < 3 )); do
    echo "while $n"
    n=$(( n + 1 ))                        # 用 $(( )) 避免 set -e 陷阱
done

m=3
until (( m == 0 )); do                    # until：条件为真时停止
    echo "until $m"
    m=$(( m - 1 ))
done

# ============================================================
# 函数（含 local）
# ============================================================
greet() {
    local target="${1:-stranger}"         # local 限定作用域
    echo "Hi, $target!"
}
greet "Bash"
greet                                     # 使用默认值 stranger

# 演示退出码（用 || 短路避免触发 set -e）
return_code() { return 4; }
return_code && echo "ok" || echo "failed with exit $?"

# ============================================================
# 数组：索引数组与关联数组
# ============================================================
fruits=("apple" "banana" "cherry")        # 索引数组
echo "first=${fruits[0]} count=${#fruits[@]}"
echo "all: ${fruits[@]}"
fruits+=("date")                          # 追加元素

declare -A person                         # 关联数组（需 declare -A）
person[name]="Alice"
person[age]=30
echo "${person[name]} is ${person[age]}"

# ============================================================
# 管道与重定向
# ============================================================
echo "line1" | tr 'a-z' 'A-Z'             # 管道
echo "log entry" >> /tmp/demo_$$.log      # 追加重定向（$$ 是当前 PID）
echo "log content: $(cat /tmp/demo_$$.log)"
rm -f /tmp/demo_$$.log

# ============================================================
# 命令行参数（$1 / $@ / $#）
# ============================================================
echo "script=$0, args_count=$#, first=$1"
for arg in "$@"; do                       # "$@" 遍历所有参数
    echo "arg: $arg"
done

# ============================================================
# heredoc（Here Document）
# ============================================================
cat <<EOF
=== Summary ===
Script : $0
Date   : $(date '+%Y-%m-%d %H:%M:%S')
Name   : $name
EOF
```
