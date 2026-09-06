---
title: Haskell 语法高亮示例
description: 以 haskell 围栏代码块渲染 haskell.hs，验证 Haskell 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Haskell 语法高亮示例

以下 `haskell.hs` 示例使用 ` ```haskell ` 围栏代码块渲染：

```haskell
-- Haskell: 纯函数式、惰性求值、静态强类型,以类型类与单子闻名

module SyntaxDemo where

-- ============ 模块导入 ============
import Data.List (sort)                  -- 显式导入列表
import qualified Data.Map.Strict as Map  -- qualified:必须用 Map.xxx 限定访问
import Data.Maybe (fromMaybe)            -- 导入特定函数

-- ============ 类型别名与 newtype ============
type Score = Int
type Name = String

-- newtype:零成本包装,用于区分相同底层类型
newtype Age = Age Int deriving (Show, Eq)

-- qualified import 使用示例
sampleMap :: Map.Map Name Score
sampleMap = Map.fromList [("Alice", 90), ("Bob", 75)]

-- ============ 代数数据类型与记录语法 ============
data Color = Red | Green | Blue deriving (Show, Eq, Enum, Bounded)

data Shape
  = Circle Double
  | Rectangle Double Double
  | Triangle Double Double Double
  deriving (Show, Eq)

-- 记录语法:自动生成 personName :: Person -> Name 等访问函数
data Person = Person
  { personName :: Name
  , personAge  :: Age
  , personAddr :: String
  } deriving (Show, Eq)

-- ============ 类型类与实例 ============
class Describable a where
  describe :: a -> String

instance Describable Color where
  describe Red   = "红色"
  describe Green = "绿色"
  describe Blue  = "蓝色"

instance Describable Shape where
  describe (Circle r)       = "圆,半径 " ++ show r
  describe (Rectangle w h)  = "矩形 " ++ show w ++ "x" ++ show h
  describe (Triangle a b c) = "三角形 " ++ show [a, b, c]

-- ============ 类型签名、函数定义、模式匹配 ============
area :: Shape -> Double
area (Circle r)       = pi * r * r
area (Rectangle w h)  = w * h
area (Triangle a b c) =
  let s = (a + b + c) / 2  -- let .. in 表达式
  in sqrt (s * (s - a) * (s - b) * (s - c))

-- 守卫表达式
classify :: Int -> String
classify n
  | n < 0     = "负数"
  | n == 0    = "零"
  | n < 100   = "小数"
  | otherwise = "大数"

-- where 子句:局部定义
bmiTell :: Double -> Double -> String
bmiTell weight height
  | ratio < 18.5 = "偏瘦"
  | ratio < 25.0 = "正常"
  | ratio < 30.0 = "超重"
  | otherwise    = "肥胖"
  where
    ratio = weight / (height ^ 2)

-- ============ 列表推导、高阶函数、lambda ============
squares :: [Int]
squares = [x * x | x <- [1 .. 10], even x]  -- 列表推导

sortedNums :: [Int]
sortedNums = sort [5, 3, 8, 1, 9]  -- 使用导入的 sort

doubled :: [Int] -> [Int]
doubled = map (* 2)

onlyPositive :: [Int] -> [Int]
onlyPositive = filter (> 0)

sumOf :: Num a => [a] -> a
sumOf = foldr (+) 0

incrementAll :: [Int] -> [Int]
incrementAll = map (\x -> x + 1)  -- lambda

-- ============ Maybe 与 Either ============
safeDiv :: Double -> Double -> Maybe Double
safeDiv _ 0 = Nothing
safeDiv x y = Just (x / y)

safeHead :: [a] -> Maybe a
safeHead []    = Nothing
safeHead (x:_) = Just x

-- fromMaybe:为 Maybe 提供默认值
headOrDefault :: a -> [a] -> a
headOrDefault def xs = fromMaybe def (safeHead xs)

parseScore :: String -> Either String Score
parseScore s = case reads s of
  [(n, "")] | n >= 0 && n <= 100 -> Right n
  _                              -> Left ("无效分数: " ++ s)

-- 列表推导中对 Either 做模式匹配,失败的匹配自动跳过
validScores :: [String] -> [Score]
validScores xs = [n | Right n <- map parseScore xs]

-- ============ Currying 与部分应用 ============
add :: Int -> Int -> Int
add x y = x + y

addFive :: Int -> Int
addFive = add 5

-- ============ IO 与 do 记法 ============
greet :: IO ()
greet = do
  putStrLn "你叫什么名字?"
  name <- getLine
  putStrLn ("你好, " ++ name ++ "!")

-- do 块内用 let(不带 in)
countLines :: IO Int
countLines = do
  contents <- getContents
  let ls = lines contents
  return (length ls)

-- ============ 主函数 ============
main :: IO ()
main = do
  putStrLn "==== Haskell 语法演示 ===="
  putStrLn ("classify 42 = " ++ classify 42)
  putStrLn ("area circle = " ++ show (area (Circle 5.0)))
  let peep = Person { personName = "Alice", personAge = Age 30, personAddr = "Earth" }
  putStrLn ("person = " ++ show peep)
  putStrLn ("describe color = " ++ describe Green)
  putStrLn ("squares = " ++ show squares)
  putStrLn ("sortedNums = " ++ show sortedNums)
  putStrLn ("sampleMap = " ++ show (Map.toList sampleMap))
  case parseScore "88" of
    Right n -> putStrLn ("parsed score = " ++ show n)
    Left e  -> putStrLn e
```
