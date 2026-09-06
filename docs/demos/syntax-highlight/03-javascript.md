---
title: JavaScript 语法高亮示例
description: 以 javascript 围栏代码块渲染 javascript.js，验证 JavaScript 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# JavaScript 语法高亮示例

以下 `javascript.js` 示例使用 ` ```javascript ` 围栏代码块渲染：

```javascript
// JavaScript (ES2020+) — 单文件现代 JS 语法速览：模块、类、异步、集合、解构等

// ============================================================
// 一、变量声明与基本类型
// ============================================================

var legacy = 'var：函数作用域，存在变量提升，现代代码尽量避免';
let mutable = 'let：块级作用域，可重新赋值';
const immutable = 'const：块级作用域，不可重新绑定（但对象内部仍可变）';

// 基本类型与 typeof
const str = 'hello';          // string
const num = 42;               // number
const big = 9007199254740993n; // bigint（整数字面量后缀 n）
const flag = true;            // boolean
const undef = undefined;      // undefined
const nothing = null;         // object（历史遗留）
const sym = Symbol('id');     // symbol，唯一不可变，常作对象键

console.log(typeof str, typeof num, typeof big, typeof flag, typeof sym);

// ============================================================
// 二、模板字符串与运算符
// ============================================================

const name = 'World';
const greeting = `Hello, ${name}! 1 + 1 = ${1 + 1}`;
console.log(greeting);

// 比较与逻辑运算符
console.log(1 == '1');  // true（宽松相等，会类型转换）
console.log(1 === '1'); // false（严格相等）
console.log(null ?? 'fallback'); // 'fallback'（空值合并）
console.log(false || 'default'); // 'default'（短路或）
console.log(true && 'both');     // 'both'

// ============================================================
// 三、解构、展开与剩余参数
// ============================================================

// 数组解构
const [first, second, ...rest] = [1, 2, 3, 4, 5];
// 对象解构 + 默认值 + 重命名
const { name: who = 'anon', age = 0 } = { name: 'Alice' };
console.log(who, age, first, rest);

// 展开数组 / 对象
const merged = [...[1, 2], ...[3, 4]];
const copied = { ...{ a: 1 }, b: 2 };

// 剩余参数（函数形参）
function sum(...nums) {
  return nums.reduce((acc, n) => acc + n, 0);
}
console.log(sum(1, 2, 3, 4)); // 10

// ============================================================
// 四、控制流与箭头函数
// ============================================================

const grade = 85;
if (grade >= 90) {
  console.log('A');
} else if (grade >= 60) {
  console.log('pass');
} else {
  console.log('fail');
}

// switch
switch (grade) {
  case 100: console.log('full'); break;
  default: console.log('ok');
}

// for...of 遍历可迭代对象
for (const item of ['a', 'b', 'c']) {
  console.log(item);
}

// 箭头函数：无自身 this，适合短回调
const square = (x) => x * x;
const noop = () => {};
[1, 2, 3].forEach((v, i) => console.log(i, v));

// ============================================================
// 五、数组方法 map / filter / reduce
// ============================================================

const nums = [1, 2, 3, 4, 5, 6];
const doubled = nums.map((n) => n * 2);          // [2,4,6,8,10,12]
const evens = nums.filter((n) => n % 2 === 0);   // [2,4,6]
const total = nums.reduce((acc, n) => acc + n, 0); // 21
console.log(doubled, evens, total);

// ============================================================
// 六、Set 与 Map
// ============================================================

const unique = new Set([1, 1, 2, 3, 3]); // {1,2,3}
const dict = new Map([['k', 'v'], ['lang', 'js']]);
dict.set('version', 2020).set('year', 2026);
for (const [key, value] of dict) {
  console.log(`${key} = ${value}`);
}

// ============================================================
// 七、类：class / extends / super
// ============================================================

class Animal {
  constructor(name) {
    this.name = name;
  }
  speak() {
    return `${this.name} makes a sound`;
  }
  static create(species) {
    return new Animal(species);
  }
}

class Dog extends Animal {
  constructor(name, breed) {
    super(name);   // 调用父类构造
    this.breed = breed;
  }
  speak() {
    return `${super.speak()} — Woof!`;
  }
}

const dog = new Dog('Rex', 'Labrador');
console.log(dog.speak());

// ============================================================
// 八、错误处理
// ============================================================

function parse(value) {
  const n = Number(value);
  if (Number.isNaN(n)) {
    throw new TypeError(`无法解析为数字: ${value}`);
  }
  return n;
}

try {
  parse('abc');
} catch (err) {
  console.error('捕获错误:', err.message);
} finally {
  console.log('finally 总会执行');
}

// ============================================================
// 九、Promise 与 async / await
// ============================================================

// 返回一个 1 秒后成功的 Promise
function delay(ms, value) {
  return new Promise((resolve) => setTimeout(() => resolve(value), ms));
}

// 链式调用
delay(100, 'step1')
  .then((v) => `${v} -> step2`)
  .then((v) => console.log('链式:', v));

// async/await 写法更接近同步代码
async function run() {
  try {
    const a = await delay(50, 'A');
    const b = await delay(50, 'B');
    // 并发执行多个 Promise
    const [x, y] = await Promise.all([delay(10, 'X'), delay(10, 'Y')]);
    console.log('async 完成:', a, b, x, y);
  } catch (err) {
    console.error('async 错误:', err);
  }
}
run();

// ============================================================
// 十、模块（import / export）
// ============================================================

// 导出命名成员（其它文件用 import { greet } from './javascript.js'）
export function greet(person) {
  return `Hi, ${person}!`;
}

// 导出常量
export const VERSION = 'ES2020+';

// 默认导出：每个模块只能有一个
export default class App {
  constructor() {
    this.startedAt = new Date();
  }
}
```
