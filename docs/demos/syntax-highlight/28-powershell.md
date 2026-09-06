---
title: PowerShell 语法高亮示例
description: 以 powershell 围栏代码块渲染 powershell.ps1，验证 PowerShell 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# PowerShell 语法高亮示例

以下 `powershell.ps1` 示例使用 ` ```powershell ` 围栏代码块渲染：

```powershell
# PowerShell — .NET 上的跨平台 shell，命令为 cmdlet，对象流过管道。

# ============================================================
# 注释：单行 # / 多行 <# ... #>
# ============================================================
# 单行注释
<#
多行注释
常用于脚本头说明或大段注解
#>

# ============================================================
# 变量、数组、哈希表
# ============================================================
$name    = "PowerShell"            # 字符串
$version = 7.4                     # 数值（double）
$count   = 42
$flag    = $true                   # 布尔：$true / $false
$empty   = $null                   # $null 即空

# 数组 @()
$colors = @("red", "green", "blue")
$range  = 1..5                     # 范围数组
$colors[0]                         # 索引（0-based）
$colors[-1]                        # 负索引：末尾元素
$colors[1..2]                      # 切片

# 哈希表 @{}
$person = @{
    Name = "Alice"
    Age  = 30
    Tags = @("dev", "lead")
}
$person["Name"]                    # 索引取值
$person.Age                        # 点号取值（属性风格）
$person.City = "NYC"               # 新增键

# ============================================================
# 字符串：单引号（原样）/ 双引号（插值）/ here-string
# ============================================================
$raw   = 'no interpolation $name'  # 单引号：原样输出
$greet = "Hello, $name!"           # 双引号：变量插值
$expr  = "2 + 3 = $($count + 3)"   # $(...) 子表达式插值
$multi = @"
here-string
spans lines, $name
"@                                 # 双引号 here-string 支持插值

# ============================================================
# cmdlet：动词-名词风格（Get-/Set-/New-/ etc.）
# ============================================================
Get-Date                           # 获取当前时间
Get-Process | Select-Object -First 3
Get-ChildItem $HOME -File | Select-Object -First 2
# New-Item -Path "demo.txt" -ItemType File -Force   # 创建文件（有副作用）

# ============================================================
# 管道 |：传递的是 .NET 对象（不是文本）
# ============================================================
Get-Service |
    Where-Object { $_.Status -eq "Running" } |
    Select-Object Name, Status |
    Sort-Object Name |
    ForEach-Object { "Service: $($_.Name)" }

# ============================================================
# 对象与属性访问
# ============================================================
$proc = Get-Process | Select-Object -First 1
$proc.Name                        # 属性访问
$proc | Get-Member                # 反射：列出类型/属性/方法
$now = Get-Date
$now.AddDays(7)                   # 调用 .NET 方法

# ============================================================
# 控制流：if / switch / for / foreach / while
# ============================================================
# 比较运算符：-gt -lt -eq -ne -ge -le（不是 > < ==）
if ($count -gt 50) {
    "big"
} elseif ($count -eq 50) {
    "equal"
} else {
    "small"
}

switch ($version) {
    { $_ -lt 7 } { "old";    break }
    { $_ -ge 7 } { "modern"; break }
    default      { "unknown" }
}

for ($i = 0; $i -lt 3; $i++) {
    "loop $i"
}

foreach ($c in $colors) {
    "color: $c"
}

$n = 0
while ($n -lt 3) { $n++ }

# ============================================================
# 函数：function / param
# ============================================================
function Greet {
    param(
        [string]$Name,
        [int]$Times = 1               # 默认值
    )
    for ($i = 1; $i -le $Times; $i++) {
        Write-Output "Hello, $Name!"
    }
}
Greet -Name "PowerShell" -Times 2

# 高级函数：[CmdletBinding] 支持管道输入
function Get-Squared {
    [CmdletBinding()]
    param(
        [Parameter(ValueFromPipeline = $true)]
        [int]$Number
    )
    process {
        Write-Output ($Number * $Number)
    }
}
1..5 | Get-Squared

# ============================================================
# 脚本块 {}：可延迟执行的代码对象
# ============================================================
$block = { param($x) "x = $x" }
& $block 42                        # & 调用脚本块
$block.Invoke(99)                  # .Invoke 方法

# ============================================================
# 错误处理：try / catch / finally / throw
# ============================================================
# 非终止错误可用 -ErrorAction Stop 转为终止错误以触发 catch
try {
    throw "explicit throw"                       # 显式抛出（终止错误）
    # Get-Item missing.txt -ErrorAction Stop    # 把 cmdlet 错误转为终止错误
} catch [System.Exception] {
    Write-Host "caught: $($_.Exception.Message)"
} finally {
    Write-Host "always runs"
}

# ============================================================
# 模块：Import-Module
# ============================================================
# Import-Module -Name ActiveDirectory            # 加载模块
# Get-Module -ListAvailable                      # 列出所有可用模块
# Get-Command -Module Microsoft.PowerShell.Management   # 列出某模块的命令
```
