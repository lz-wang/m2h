---
title: C# 语法高亮示例
description: 以 csharp 围栏代码块渲染 csharp.cs，验证 C# 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# C# 语法高亮示例

以下 `csharp.cs` 示例使用 ` ```csharp ` 围栏代码块渲染：

```csharp
// C# 语言：运行在 .NET 上的现代面向对象语言，支持 LINQ、async/await 与强类型。
using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;

namespace Demo
{
    // ===== 接口 =====
    public interface IShape
    {
        double Area();
    }

    // ===== 类：继承、自动属性、表达式体构造 =====
    public class Circle : IShape
    {
        public double Radius { get; set; } // 自动属性

        public Circle(double r) => Radius = r; // 表达式体构造函数

        public double Area() => 3.14159 * Radius * Radius; // 表达式体方法
    }

    public class Rectangle : IShape
    {
        public double Width { get; set; }
        public double Height { get; set; }

        public double Area() => Width * Height;
    }

    // ===== 泛型方法 + 约束 =====
    public static class Helpers
    {
        public static T Max<T>(T a, T b) where T : IComparable<T>
            => a.CompareTo(b) >= 0 ? a : b;
    }

    internal class Program
    {
        // ===== async Main =====
        private static async Task Main(string[] args)
        {
            // 变量与基本类型
            int i = 10;
            double d = 3.14;
            bool flag = true;
            string text = "C#";
            var inferred = 42; // var 类型推断

            // 运算符
            int sum = i + 5;
            bool both = flag && i > 0;
            Console.WriteLine($"sum={sum} both={both} text={text} inferred={inferred}");

            // 控制流：if / else
            if (i % 2 == 0)
                Console.WriteLine("even");
            else
                Console.WriteLine("odd");

            // ===== switch 表达式（模式匹配） =====
            string label = i switch
            {
                < 0 => "negative",
                0 => "zero",
                _ => "positive"
            };
            Console.WriteLine(label);

            // for / while
            for (int k = 0; k < 3; k++) Console.Write(k + " ");
            Console.WriteLine();
            int n = 3;
            while (n > 0) n--;

            // 方法调用
            Console.WriteLine("Add(3,4)=" + Add(3, 4));

            // 多态：接口指向派生类
            IShape s = new Circle(2);
            Console.WriteLine($"area={s.Area():F2}");

            // 泛型集合
            var list = new List<int> { 5, 3, 9, 1 };
            var dict = new Dictionary<string, int>
            {
                ["alice"] = 90,
                ["bob"] = 85
            };

            // ===== LINQ =====
            var sorted = list.OrderBy(x => x).ToList();
            int evensSum = list.Where(x => x % 2 == 0).Sum();
            Console.WriteLine($"sorted={string.Join(",", sorted)} evensSum={evensSum}");

            // 异常：try / catch / finally
            try
            {
                int[] arr = { 1, 2 };
                Console.WriteLine(arr[5]); // 越界
            }
            catch (IndexOutOfRangeException ex)
            {
                Console.WriteLine("caught: " + ex.Message);
            }
            finally
            {
                Console.WriteLine("finally block");
            }

            // ===== async / await =====
            int result = await FetchAsync();
            Console.WriteLine($"async result={result}");
        }

        // 表达式体方法
        static int Add(int a, int b) => a + b;

        // 异步方法
        static async Task<int> FetchAsync()
        {
            await Task.Delay(100); // 模拟异步 I/O
            return 42;
        }
    }
}
```
