---
title: TypeScript 语法高亮示例
description: 以 typescript 围栏代码块渲染 typescript.ts，验证 TypeScript 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# TypeScript 语法高亮示例

以下 `typescript.ts` 示例使用 ` ```typescript ` 围栏代码块渲染：

```typescript
// TypeScript — 在 JavaScript 之上添加静态类型系统

// ============================================================
// 一、基本类型注解（变量 / 参数 / 返回值）
// ============================================================

const count: number = 10;
const label: string = 'TS';
const done: boolean = false;

// 函数参数与返回值类型
function add(a: number, b: number): number {
  return a + b;
}

// ============================================================
// 二、接口 interface 与 类型别名 type
// ============================================================

interface User {
  readonly id: number;       // 只读属性
  name: string;
  age?: number;              // 可选属性
}

type Point = { x: number; y: number };

const u: User = { id: 1, name: 'Alice' };
const p: Point = { x: 3, y: 4 };

// ============================================================
// 三、联合类型、交叉类型、字面量类型
// ============================================================

// 联合：值可以是其中之一
type ID = number | string;

// 字面量类型：值只能是若干固定字面量
type Status = 'idle' | 'loading' | 'success' | 'error';

// 交叉类型：把多个类型合并
type WithTimestamp = { createdAt: string };
type AuditedUser = User & WithTimestamp;

let current: Status = 'loading';
const audited: AuditedUser = { id: 2, name: 'Bob', createdAt: '2026-01-01' };

// ============================================================
// 四、元组 Tuple
// ============================================================

const tuple: [string, number] = ['age', 30];
const [key, value] = tuple;

// ============================================================
// 五、枚举 enum
// ============================================================

enum Color {
  Red = 0,
  Green = 1,
  Blue = 2,
}
const c: Color = Color.Green;

// ============================================================
// 六、泛型（函数 / 接口）
// ============================================================

// 泛型函数：T 是类型参数
function first<T>(arr: T[]): T | undefined {
  return arr[0];
}
const n = first<number>([1, 2, 3]); // number | undefined

// 泛型接口
interface Box<T> {
  value: T;
}
const box: Box<string> = { value: 'hello' };

// 泛型约束（extends 限定上界）
function lengthOf<T extends { length: number }>(item: T): number {
  return item.length;
}
console.log(lengthOf('abc'), lengthOf([1, 2]));

// ============================================================
// 七、类与访问修饰符
// ============================================================

class Account {
  public id: number;          // 公开，默认
  private balance: number;    // 私有，仅类内可访问
  protected owner: string;    // 受保护，子类也可访问
  readonly createdAt: string; // 只读

  constructor(id: number, owner: string) {
    this.id = id;
    this.owner = owner;
    this.balance = 0;
    this.createdAt = new Date().toISOString();
  }

  deposit(amount: number): void {
    this.balance += amount;
  }

  getBalance(): number {
    return this.balance;
  }
}

class SavingsAccount extends Account {
  constructor(id: number, owner: string, public rate: number) {
    super(id, owner); // 调用父类构造
  }
}

// ============================================================
// 八、函数类型
// ============================================================

// 显式函数类型
type MathOp = (a: number, b: number) => number;
const multiply: MathOp = (a, b) => a * b;

// ============================================================
// 九、类型断言 as、unknown、never
// ============================================================

// unknown：安全的 any，使用前必须收窄
function process(input: unknown): string {
  if (typeof input === 'string') {
    return input.toUpperCase();
  }
  return String(input);
}

// 类型断言：告诉编译器更具体的类型
const el = document.querySelector('#app') as HTMLDivElement;

// never：永不到达的返回类型，常用于穷尽检查
function fail(msg: string): never {
  throw new Error(msg);
}

function handle(s: Status): string {
  switch (s) {
    case 'idle':    return '等待';
    case 'loading': return '加载中';
    case 'success': return '成功';
    case 'error':   return '出错';
    default:
      // 漏写分支时 s 类型会变成对应字面量，编译报错
      const _exhaustive: never = s;
      return _exhaustive;
  }
}

// ============================================================
// 十、工具类型（内置）
// ============================================================

interface Todo {
  title: string;
  description: string;
  done: boolean;
}

type PartialTodo = Partial<Todo>;              // 所有属性可选
type ReadonlyTodo = Readonly<Todo>;            // 所有属性只读
type TodoPreview = Pick<Todo, 'title' | 'done'>; // 只挑部分键
type TodoMeta = Omit<Todo, 'description'>;     // 去掉部分键
type TodoMap = Record<string, Todo>;           // 键值映射

const draft: PartialTodo = { title: 'Learn TS' };
const preview: TodoPreview = { title: 'Demo', done: false };

// ============================================================
// 十一、模块导出 / 导入
// ============================================================

export { User, Point, add, type ID, type Status };

export default class Config {
  constructor(public env: 'dev' | 'prod') {}
}

// 在其它文件：
//   import Config, { User, type ID } from './typescript';
```
