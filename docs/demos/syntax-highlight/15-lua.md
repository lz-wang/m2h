---
title: Lua 语法高亮示例
description: 以 lua 围栏代码块渲染 lua.lua，验证 Lua 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Lua 语法高亮示例

以下 `lua.lua` 示例使用 ` ```lua ` 围栏代码块渲染：

```lua
-- Lua — 轻量、快速、可嵌入的脚本语言，广泛用于游戏与配置。

-- ============================================================
-- 注释：单行用 --，多行用 --[[ ]]
-- ============================================================
--[[
这是多行注释。
Lua 默认只有一种数据结构：表（table）。
]]

-- ============================================================
-- 变量：local（局部）与全局（默认）
-- ============================================================
local x = 10
y = 20            -- 全局变量（慎用）
local a, b = 1, 2 -- 多重赋值

-- ============================================================
-- 8 种基本类型
-- ============================================================
local s = "hello"                                      -- string
local n = 3.14                                         -- number（整数/浮点统一）
local flag = true                                      -- boolean
local nothing = nil                                    -- nil（空）
local f = function() end                               -- function
local t = {}                                           -- table
local co = coroutine.create(function() end)            -- thread（协程）
local u = io.read                                      -- userdata（C 数据）

print(type(s), type(n), type(t))   -- -> string  number  table

-- ============================================================
-- 运算符（注意：不等于用 ~=，逻辑用 and/or/not）
-- ============================================================
print(7 // 2)        -- 3（整除，Lua 5.3+）
print(2 ^ 10)        -- 1024.0（幂）
print(10 % 3)        -- 1（取模）
print(nil and 1)     -- nil（短路）
print(false or "x")  -- "x"
print(1 ~= 2)        -- true

-- 字符串拼接用 ..（不是 +），# 取长度
local greeting = "Hello, " .. "Lua"
print(#greeting)     -- -> 11

-- ============================================================
-- 控制流：if / for（数值与泛型）/ while / repeat
-- ============================================================
local age = 20
if age < 13 then
    print("child")
elseif age < 18 then
    print("teen")
else
    print("adult")
end

-- 数值型 for
for i = 1, 3 do print(i) end          -- 1 2 3
for i = 10, 1, -2 do print(i) end     -- 10 8 6 4 2

-- 泛型 for：ipairs 遍历序列，pairs 遍历任意 table
local colors = {"red", "green", "blue"}
for index, value in ipairs(colors) do
    print(index, value)
end
local dict = {name = "Lua", year = 1993}
for key, value in pairs(dict) do
    print(key, value)
end

-- while 与 repeat...until（until 条件为真时停止）
local c = 0
while c < 3 do c = c + 1 end
repeat
    c = c - 1
until c == 0

-- ============================================================
-- 函数：多返回值 / 可变参数 ...
-- ============================================================
local function minmax(...)
    local args = {...}                       -- 打包为 table
    local mn, mx = args[1], args[1]
    for _, v in ipairs(args) do
        if v < mn then mn = v end
        if v > mx then mx = v end
    end
    return mn, mx                            -- 多返回值
end

local lo, hi = minmax(3, 1, 4, 1, 5, 9)
print(lo, hi)                                -- -> 1  9

-- ============================================================
-- 表 table：数组 / 字典 / 对象表示（用 table + 元表模拟类）
-- ============================================================
local Point = {}
Point.__index = Point                        -- 元方法：找不到字段时回退到 Point

function Point.new(x, y)                      -- 构造函数
    local self = setmetatable({}, Point)
    self.x = x or 0
    self.y = y or 0
    return self
end

function Point:distance(other)                -- 冒号 = 隐式 self 参数
    local dx = self.x - other.x
    local dy = self.y - other.y
    return math.sqrt(dx * dx + dy * dy)
end

local p1 = Point.new(0, 0)
local p2 = Point.new(3, 4)
print(p1:distance(p2))                        -- -> 5.0

-- ============================================================
-- 元表与元方法（__index / __newindex）
-- ============================================================
local defaults = {x = 0, y = 0, color = "black"}
local obj = setmetatable({}, {__index = defaults})
print(obj.x, obj.color)                       -- 借助 __index 取默认值

local protected = setmetatable({}, {
    __newindex = function(tab, key, val)      -- 拦截新键写入
        rawset(tab, key, val)
        print("set " .. key .. " = " .. tostring(val))
    end,
})
protected.name = "Lua"                        -- 触发 __newindex

-- ============================================================
-- 闭包
-- ============================================================
local function counter()
    local n = 0
    return function()                         -- 闭包捕获外层 n
        n = n + 1
        return n
    end
end
local nxt = counter()
print(nxt(), nxt(), nxt())                    -- -> 1 2 3

-- ============================================================
-- 协程（coroutine）
-- ============================================================
local function producer()
    for i = 1, 3 do
        coroutine.yield(i)                    -- 挂起并传出值
    end
end
local co = coroutine.create(producer)
print(coroutine.resume(co))                   -- -> true 1
print(coroutine.resume(co))                   -- -> true 2
print(coroutine.resume(co))                   -- -> true 3
print(coroutine.resume(co))                   -- -> true（协程结束）

-- ============================================================
-- 模块（require）
-- ============================================================
-- 真实项目中：在模块文件里 local M = {} ... return M，
-- 然后 local M = require("module_name") 引入。
print(math.max(1, 2, 3), string.upper("hi"))  -- 使用标准库模块
```
