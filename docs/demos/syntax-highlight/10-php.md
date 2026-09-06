---
title: PHP 语法高亮示例
description: 以 php 围栏代码块渲染 php.php，验证 PHP 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# PHP 语法高亮示例

以下 `php.php` 示例使用 ` ```php ` 围栏代码块渲染：

```php
<?php
// PHP — 服务端 Web 开发常用语言，可嵌入 HTML。

// ============================================================
// 数据类型与变量（变量以 $ 开头）
// ============================================================
$int     = 42;
$float   = 3.14;
$string  = "Hello";
$bool    = true;
$null    = null;
$indexed = [1, 2, 3];                          // 索引数组
$assoc   = ['name' => 'Alice', 'age' => 30];   // 关联数组

// 字符串与 heredoc
$greeting = "Hi, $string";                      // 双引号插值
$raw      = 'no $interpolation here';           // 单引号原样
$doc      = <<<EOT
多行字符串（heredoc），
变量 \$int = $int 可在此插值。
EOT;

// ============================================================
// 运算符（含太空船运算符 <=>）
// ============================================================
echo (2 <=> 5);   // -1：左 < 右
echo (5 <=> 5);   //  0：相等
echo (7 <=> 2);   //  1：左 > 右

// 数组运算符
$merged = ['a'] + ['b'];        // 联合
$equal  = [1, 2] === [1, 2];    // 全等比较 -> true

// null 合并运算符 ??（PHP 7+）
$id = $_GET['id'] ?? 'default';

// ============================================================
// 控制流：if / switch / foreach
// ============================================================
if ($int > 50) {
    echo "big";
} elseif ($int > 10) {
    echo "medium";
} else {
    echo "small";
}

switch ($string) {
    case "Hi":
    case "Hello":
        echo "matched";
        break;
    default:
        echo "default";
}

// foreach 遍历关联与索引数组
foreach ($assoc as $key => $value) {
    echo "$key => $value\n";
}
foreach ($indexed as $item) {
    echo $item;
}

// ============================================================
// 函数：默认参数 / 类型声明 / 可变参数
// ============================================================
function add(int $a, int $b = 10): int {        // 类型声明 + 默认值
    return $a + $b;
}

function sumAll(int ...$nums): int {            // 可变参数 ...
    return array_sum($nums);
}

// 闭包（匿名函数）
$multiplier = fn($x) => $x * 3;                 // 箭头函数（PHP 7.4+）
echo add(5);                                    // -> 15
echo sumAll(1, 2, 3);                           // -> 6
echo $multiplier(4);                            // -> 12

// ============================================================
// 类：属性 / 可见性 / 继承 / 接口 / trait / 静态
// ============================================================
interface Greeter {
    public function greet(): string;            // 接口契约
}

trait Timestamps {                              // trait：方法复用
    public function createdAt(): string {
        return date('Y-m-d');
    }
}

class Animal {
    public string $name;                        // 可见性 + 类型属性
    protected int $age;
    private string $secret;

    public function __construct(string $name, int $age = 0) {
        $this->name = $name;
        $this->age = $age;
        $this->secret = 'hidden';               // $this 访问成员
    }

    public static function species(): string {  // 静态方法
        return 'unknown';
    }
}

class Dog extends Animal implements Greeter {   // 继承 + 实现接口
    use Timestamps;                             // 使用 trait

    public function greet(): string {           // 实现接口方法
        return "{$this->name}: Woof!";
    }
}

$dog = new Dog('Rex', 3);
echo $dog->greet();                             // -> "Rex: Woof!"
echo $dog->createdAt();                         // 来自 trait
echo Dog::species();                            // 静态调用

// ============================================================
// 命名空间（namespace 必须是文件首条语句，此处为示例）
// ============================================================
// 真实项目中通常写在文件最顶部：
//   namespace App\Service;
//   use App\Model\User;
//   class UserService {
//       public function find(int $id): ?User { ... }
//   }
// 类引用需配合 use 导入；全局函数/类可用 \ 前缀，如 \count()。

// ============================================================
// 异常：try / catch / finally
// ============================================================
function divide(int $a, int $b): float {
    if ($b === 0) {
        throw new \InvalidArgumentException('Division by zero');
    }
    return $a / $b;
}

try {
    echo divide(10, 2);
} catch (\InvalidArgumentException $e) {
    echo "Error: " . $e->getMessage();
} finally {
    echo "cleanup";
}

// ============================================================
// 超全局变量（简要）
// ============================================================
// $_GET  / $_POST    —— HTTP 请求参数
// $_SERVER           —— 服务器与执行环境信息
// $_COOKIE / $_SESSION —— Cookie 与会话
// $_FILES            —— 上传文件
$input = $_POST['data'] ?? '';                  // 配合 ?? 处理未定义键
?>
```
