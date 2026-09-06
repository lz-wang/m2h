---
title: C++ 语法高亮示例
description: 以 cpp 围栏代码块渲染 cpp.cpp，验证 C++ 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# C++ 语法高亮示例

以下 `cpp.cpp` 示例使用 ` ```cpp ` 围栏代码块渲染：

```cpp
// C++ 现代风格：RAII、智能指针、模板、lambda、STL 容器与异常的通用语言。
#include <iostream>
#include <vector>
#include <map>
#include <string>
#include <array>
#include <memory>
#include <algorithm>
#include <stdexcept>
#include <cstddef>

// ===== 命名空间 =====
namespace app {

// ===== 模板函数 =====
template <typename T>
T maxValue(const std::vector<T>& v) {
    T m = v.at(0);
    for (const auto& x : v) {
        if (x > m) m = x;
    }
    return m;
}

// ===== 类：继承与多态（virtual） =====
class Animal {
public:
    virtual std::string sound() const { return "..."; }
    virtual ~Animal() = default; // 虚析构，保证正确释放
};

class Dog : public Animal {
public:
    std::string sound() const override { return "Woof"; }
};

// ===== 构造/析构演示（RAII） =====
class Counter {
    int n_;

public:
    explicit Counter(int n) : n_(n) {
        std::cout << "ctor n=" << n_ << "\n";
    }
    ~Counter() { std::cout << "dtor n=" << n_ << "\n"; }
    int get() const { return n_; }
};

} // namespace app

// ===== 模板类 =====
template <typename T, std::size_t N>
struct Stack {
    std::array<T, N> data;
    std::size_t top = 0;
    void push(const T& v) { data[top++] = v; }
    T pop() { return data[--top]; }
};

int main() {
    using namespace app;

    // auto 类型推导
    auto pi = 3.14;
    auto msg = std::string("modern C++");
    std::cout << msg << " " << pi << "\n";

    // 控制流：范围 for
    std::vector<int> nums = {3, 1, 4, 1, 5, 9};
    for (auto x : nums) std::cout << x << " ";
    std::cout << "\n";

    // STL 算法
    std::sort(nums.begin(), nums.end());

    // map（红黑树有序映射）
    std::map<std::string, int> ages;
    ages["Alice"] = 30;
    ages["Bob"] = 25;
    for (const auto& [name, age] : ages) {
        std::cout << name << ":" << age << " ";
    }
    std::cout << "\n";

    // lambda 表达式
    auto add = [](int a, int b) { return a + b; };
    std::cout << "1+2=" << add(1, 2) << "\n";

    // 多态 + unique_ptr（独占所有权）
    std::unique_ptr<Animal> a = std::make_unique<Dog>();
    std::cout << a->sound() << "\n";

    // shared_ptr（引用计数共享所有权）
    auto sp = std::make_shared<Counter>(5);
    std::cout << sp->get() << "\n";

    // 模板函数
    std::cout << "max=" << maxValue(nums) << "\n";

    // 模板类
    Stack<int, 8> st;
    st.push(10);
    st.push(20);
    std::cout << "pop=" << st.pop() << "\n";

    // 异常处理：try / catch / throw
    try {
        if (nums.empty()) throw std::runtime_error("empty");
        std::cout << "first=" << nums.at(0) << "\n";
    } catch (const std::exception& e) {
        std::cerr << "error: " << e.what() << "\n";
    }

    return 0; // sp 等局部对象在此自动析构（RAII）
}
```
