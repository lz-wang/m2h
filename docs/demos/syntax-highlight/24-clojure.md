---
title: Clojure 语法高亮示例
description: 以 clojure 围栏代码块渲染 clojure.clj，验证 Clojure 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Clojure 语法高亮示例

以下 `clojure.clj` 示例使用 ` ```clojure ` 围栏代码块渲染：

```clojure
;; Clojure: 运行在 JVM 上的 Lisp 方言,不可变持久数据结构 + 函数式 + STM 并发

(ns syntaxdemo.core
  (:require [clojure.string :as str])
  (:import [java.util Date UUID]))

;; ============ 数据结构字面量 ============
(def a-list '(1 2 3))              ;; 列表
(def a-vec [1 2 3])                ;; 向量
(def a-map {:a 1 :b 2 :c 3})       ;; map,关键字作键
(def a-set #{:x :y :z})            ;; set

;; ============ 关键字 ============
;; 关键字以 : 开头,求值等于自身,常作 map 的键
(def kw :user/name)
(def ns-kw ::name)                 ;; 带当前命名空间的关键字

;; ============ def 与 defn ============
(def pi 3.14159)

(defn square [x]                   ;; 普通函数
  (* x x))

(defn greet                        ;; 多元数(multi-arity)
  ([name] (str "Hello, " name))
  ([name lang] (str "Hi " name " (" lang ")")))

(defn bmi                          ;; 文档字符串 + 前置条件
  "计算 BMI,要求输入为正数"
  [weight height]
  {:pre [(pos? weight) (pos? height)]}
  (/ weight (* height height)))

;; ============ 匿名函数 ============
(def inc-1 (fn [x] (+ x 1)))       ;; fn 形式
(def inc-2 #(+ % 1))               ;; #() 读取器简写,% 为首个参数

;; ============ let 与解构 ============
(let [[a b c] [1 2 3]
      {:keys [name age]} {:name "Alice" :age 30}]
  (println name age a))

;; ============ loop-recur(尾递归)============
(defn factorial [n]
  (loop [i n acc 1]
    (if (zero? i)
      acc
      (recur (dec i) (* acc i)))))

;; ============ 序列与高阶函数 ============
(def nums [1 2 3 4 5])

(def sq (map #(* % %) nums))              ;; map
(def evens (filter even? nums))           ;; filter
(def total (reduce + 0 nums))             ;; reduce

;; seq:惰性序列
(def naturals (range))                    ;; 无限序列
(def first-ten (take 10 naturals))

;; ============ map / vector 操作 ============
(assoc {:a 1} :b 2)                       ;; => {:a 1, :b 2}
(dissoc {:a 1 :b 2} :a)                   ;; => {:b 2}
(get {:a 1} :a)                           ;; => 1
(:a {:a 1})                               ;; 关键字可作为函数
(conj [1 2] 3)                            ;; => [1 2 3]
(cons 0 [1 2])                            ;; => (0 1 2)

;; ============ defrecord 与 defprotocol ============
(defprotocol Greetable
  (greet [this]))

(defrecord User [name email]
  Greetable
  (greet [_] (str "Hi, " name)))

(def u (->User "Bob" "bob@example.com"))
;; (greet u) => "Hi, Bob"

;; ============ 宏(quote 简要)============
(defmacro unless [pred & body]
  `(if (not ~pred) (do ~@body) nil))
;; ' 阻止求值;` 语法-quote;~ unquote;~@ splice-unquote
;; (unless false (println "ok")) => 打印 ok

;; ============ 原子 atom 与 引用 ref(简要)============
(def counter (atom 0))

(swap! counter inc)                       ;; 原子地应用函数
(deref counter)                           ;; => 1,等价于 @counter

(def balance (ref 100))
(dosync
  (alter balance + 50))                   ;; 在事务中更新 ref

;; ============ Java 互操作 ============
(def now (Date.))                         ;; 构造对象:new Date()
(def ms (.getTime now))                   ;; 实例方法
(def uuid (.toString (UUID/randomUUID)))  ;; 静态方法

;; ============ 主入口 ============
(defn -main [& args]
  (println "==== Clojure 语法演示 ====")
  (println "factorial 5 =" (factorial 5))
  (println "squares =" sq)
  (println "greet alice =" (greet "Alice"))
  (println "user record =" u)
  (println "counter =" @counter))
```
