---
title: Perl 语法高亮示例
description: 以 perl 围栏代码块渲染 perl.pl，验证 Perl 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Perl 语法高亮示例

以下 `perl.pl` 示例使用 ` ```perl ` 围栏代码块渲染：

```perl
#!/usr/bin/perl
# Perl — 强大的文本处理语言，以正则表达式、列表与魔法变量著称。

use strict;        # 强制 my 声明，强烈推荐
use warnings;      # 编译期警告
# use v5.36;       # 启用现代特性（隐含 strict/warnings，并提供 signatures）

# ============================================================
# 标量 $ / 数组 @ / 哈希 %
# ============================================================
my $name  = "Perl";                    # 标量：字符串
my $count = 42;                        # 标量：数字（Perl 内部不区分 int/float）
my $pi    = 3.14159;

my @colors = ("red", "green", "blue"); # 数组（有序）
my %person = (                         # 哈希（键值对）
    name => "Alice",
    age  => 30,
);

# ============================================================
# 上下文：标量上下文 vs 列表上下文（Perl 的核心概念）
# ============================================================
my @arr = (1, 2, 3, 4, 5);

my $len = @arr;                        # 标量上下文 -> 5（数组长度）
my @copy = @arr;                       # 列表上下文 -> 拷贝整个数组
my ($first, $second) = @arr;           # 列表赋值 -> 1, 2

# reverse 的双面性
my @rev_list = reverse @arr;           # 列表上下文 -> (5, 4, 3, 2, 1)
my $rev_str  = reverse "abc";          # 标量上下文 -> "cba"
my $rev_num  = scalar reverse @arr;    # 强制标量 -> "54321"

# ============================================================
# 引用（reference）：构造复杂数据结构的关键
# ============================================================
my $arr_ref  = \@arr;                  # 数组引用
my $hash_ref = \%person;               # 哈希引用
my $anon_arr  = [1, 2, [3, 4]];        # 匿名数组（生成引用）
my $anon_hash = { a => 1, b => 2 };    # 匿名哈希（生成引用）

# 解引用
print $arr_ref->[0], "\n";             # 箭头解引用（推荐）
print $hash_ref->{name}, "\n";
print $anon_arr->[2][0], "\n";         # 链式访问嵌套
my @inner = @{$anon_arr->[2]};         # 整块解引用 @{ }

# ============================================================
# 控制流：if / unless / while / until / for / foreach
# ============================================================
if ($count > 50) {
    print "big\n";
} elsif ($count == 50) {
    print "equal\n";
} else {
    print "small\n";
}

print "ok\n" if $count > 0;            # 后缀 if（语句修饰符）
print "skipped\n" unless $count < 0;   # unless = if not

my $i = 0;
while ($i < 3) { $i++; }               # 当条件为真
my $j = 3;
until ($j == 0) { $j--; }              # 直到条件为真（while 的否定）

# for 与 foreach 同义
for my $n (0 .. 5) {
    next if $n % 2 == 0;               # next 跳过本次
    last if $n > 4;                    # last 退出循环
    print "$n ";
}
print "\n";

# ============================================================
# 特殊变量：$_ / @_ / @ARGV / $0 / $!
# ============================================================
for ("a", "b", "c") {                  # 默认绑定到 $_
    print;                              # print 无参数时输出 $_
    print "\n";
}

print "Script args: @ARGV\n";          # 命令行参数
print "Script name: $0\n";             # 脚本名
# open(my $fh, "<", "x") or die "cannot: $!";   # $! = 系统错误信息

# ============================================================
# 正则表达式：m// / s/// / tr///
# ============================================================
my $text = "Hello, World! 123";

# 匹配 m//（m 可省略 /.../）
if ($text =~ m/\d+/) { print "has digits\n"; }
if ($text =~ /^Hello/) { print "starts with Hello\n"; }

# 捕获与命名捕获
if ($text =~ /(\w+), (\w+)!/) {
    print "word1=$1, word2=$2\n";      # $1 $2 为捕获变量
}
# 命名捕获：$+{year}
"text 2024" =~ /(?<year>\d{4})/ and print "year=$+{year}\n";

# 替换 s///
my $clean = $text;
$clean =~ s/\d+//g;                    # 删除所有数字（g 全局）
$clean =~ s/\s+$//;                    # 去除尾部空白

# 字符转换 tr///（按字符表翻译，非正则）
my $upper = "abc";
$upper =~ tr/a-z/A-Z/;                 # -> "ABC"
my $vowel_count = ($text =~ tr/aeiouAEIOU//);   # 统计元音数量
print "vowels=$vowel_count\n";

# ============================================================
# 子程序：sub / @_ / 多返回值
# ============================================================
sub greet {
    my ($name) = @_;                   # 解包参数（推荐写法）
    return "Hello, $name!";
}

sub stats {                            # 返回多个值（列表）
    my @nums = @_;
    my @sorted = sort { $a <=> $b } @nums;
    return ($sorted[0], $sorted[-1], scalar @nums);
}
my ($lo, $hi, $cnt) = stats(3, 1, 4, 1, 5, 9);

# 子程序签名（v5.20+，需 use feature 'signatures' 或 v5.36）
# use feature 'signatures';
# sub add($a, $b) { return $a + $b }

# ============================================================
# grep / map：列表处理利器
# ============================================================
my @nums = (1 .. 10);
my @evens   = grep { $_ % 2 == 0 } @nums;     # 筛选
my @squares = map  { $_ * $_ }        @nums;   # 变换
my $sum = 0;
$sum += $_ for @nums;                          # 后缀循环累加

# ============================================================
# 文件句柄：open / 读 / 写（三参数形式推荐）
# ============================================================
# 读取（逐行）
# open(my $fh, "<", "input.txt") or die "cannot open: $!";
# while (my $line = <$fh>) {        # <$fh> 读一行
#     chomp $line;                   # 去掉行尾换行
#     print "line: $line\n";
# }
# close $fh;

# 写入（注意 print 后无逗号，句柄紧跟）
# open(my $wh, ">", "output.txt") or die "cannot write: $!";
# print $wh "log entry\n";
# close $wh;

# ============================================================
# 包（package）与模块（use）
# ============================================================
# 一个 .pm 模块文件通常这样组织：
#   package Greeter;
#   use strict; use warnings;
#   sub hello { my ($n) = @_; return "hi $n" }
#   1;   # 模块文件必须以真值结尾，供 require 判定加载成功
#
# 使用方：
#   use Greeter;             # 编译期加载并导入
#   print Greeter::hello("World"), "\n";

# 同一脚本内也可定义多个 package
package Greeter;
sub hello {
    my ($who) = @_;
    return "Greeter says hi to $who";
}

package main;                          # 切回主包
print Greeter::hello("World"), "\n";

# use 加载标准库模块（编译期）
use File::Basename qw(fileparse);
my ($base, $dir, $suffix) = fileparse("/tmp/demo.txt", qr/\.[^.]*/);
print "base=$base\n";
```
