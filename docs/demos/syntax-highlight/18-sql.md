---
title: SQL 语法高亮示例
description: 以 sql 围栏代码块渲染 sql.sql，验证 SQL 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# SQL 语法高亮示例

以下 `sql.sql` 示例使用 ` ```sql ` 围栏代码块渲染：

```sql
-- SQL：声明式关系数据库查询语言（PostgreSQL 风格，可读无需建库）
/* 块注释：本文件演示 DDL / DML / 查询 / 事务 / 窗口函数等核心语法 */

-- ========== DDL：建表与约束 ==========
CREATE TABLE departments (
    id      SERIAL PRIMARY KEY,            -- 自增主键（PostgreSQL）
    name    VARCHAR(50) NOT NULL UNIQUE,   -- 非空 + 唯一
    budget  NUMERIC(12,2) DEFAULT 0 CHECK (budget >= 0)  -- 默认值 + 检查约束
);

CREATE TABLE employees (
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(100) NOT NULL,
    email      VARCHAR(120) UNIQUE,                       -- 唯一约束
    salary     NUMERIC(10,2) NOT NULL CHECK (salary > 0), -- 检查约束
    dept_id    INTEGER REFERENCES departments(id),        -- 外键
    hired_at   DATE DEFAULT CURRENT_DATE,                 -- 默认值
    is_active  BOOLEAN DEFAULT TRUE
);

-- 索引（加速查询）
CREATE INDEX idx_emp_dept ON employees(dept_id);
CREATE UNIQUE INDEX idx_emp_email ON employees(email);

-- ALTER（简要：增删列）
ALTER TABLE employees ADD COLUMN note TEXT;
ALTER TABLE employees DROP COLUMN note;

-- DROP（简要，CASCADE 级联删除依赖）
-- DROP TABLE employees;
-- DROP TABLE departments CASCADE;

-- ========== DML：增删改 ==========
INSERT INTO departments (name, budget) VALUES
    ('Engineering', 1000000),
    ('Sales', 500000);

INSERT INTO employees (name, email, salary, dept_id)
VALUES ('Alice', 'alice@ex.com', 90000, 1),
       ('Bob',   'bob@ex.com',   75000, 1),
       ('Carol', 'carol@ex.com', 80000, 2);

UPDATE employees SET salary = salary * 1.1 WHERE name = 'Alice';

DELETE FROM employees WHERE is_active = FALSE;

-- ========== SELECT：列、别名、条件、排序、分页 ==========
SELECT e.id AS emp_id,
       e.name AS emp_name,
       e.salary * 12 AS yearly          -- 表达式 + 别名
FROM employees e
WHERE e.salary >= 70000 AND e.is_active = TRUE
ORDER BY yearly DESC, emp_name ASC
LIMIT 10 OFFSET 0;                       -- 分页

-- 运算符：IN / BETWEEN / LIKE / IS NULL
SELECT name FROM employees
WHERE dept_id IN (1, 2)
  AND salary BETWEEN 50000 AND 100000
  AND email IS NOT NULL
  AND name LIKE 'A%';

-- ========== 聚合 + GROUP BY + HAVING ==========
SELECT d.name AS dept,
       COUNT(*)        AS headcount,
       AVG(e.salary)   AS avg_salary,
       MAX(e.salary)   AS max_salary,
       MIN(e.salary)   AS min_salary,
       SUM(e.salary)   AS total_salary
FROM employees e
JOIN departments d ON e.dept_id = d.id
GROUP BY d.name
HAVING AVG(e.salary) > 70000             -- 分组后再过滤
ORDER BY avg_salary DESC;

-- ========== JOIN：INNER 与 LEFT ==========
-- INNER JOIN：只保留两表都匹配的行
SELECT e.name, d.name AS dept
FROM employees e
INNER JOIN departments d ON e.dept_id = d.id;

-- LEFT JOIN：保留左表全部行，右表无匹配则为 NULL
SELECT e.name, d.name AS dept
FROM employees e
LEFT JOIN departments d ON e.dept_id = d.id;

-- ========== 子查询 ==========
-- 非相关子查询
SELECT name, salary
FROM employees
WHERE salary > (SELECT AVG(salary) FROM employees);

-- 相关子查询（引用外层字段）
SELECT e.name
FROM employees e
WHERE e.salary > (
    SELECT AVG(e2.salary)
    FROM employees e2
    WHERE e2.dept_id = e.dept_id
);

-- ========== CTE：通用表表达式（WITH） ==========
WITH dept_avg AS (
    SELECT dept_id, AVG(salary) AS avg_sal
    FROM employees
    GROUP BY dept_id
)
SELECT e.name, e.salary, da.avg_sal
FROM employees e
JOIN dept_avg da ON e.dept_id = da.dept_id
WHERE e.salary > da.avg_sal;

-- ========== 视图（CREATE VIEW，简要） ==========
CREATE VIEW high_paid AS
SELECT id, name, salary FROM employees WHERE salary > 80000;

-- SELECT * FROM high_paid;

-- ========== 事务（BEGIN / COMMIT / ROLLBACK，简要） ==========
BEGIN;
UPDATE employees SET salary = salary + 1000 WHERE name = 'Bob';
-- 出错可回滚：ROLLBACK;
COMMIT;

-- ========== 窗口函数（ROW_NUMBER() OVER，简要） ==========
-- ROW_NUMBER：按部门薪资排名（PARTITION BY 分区，ORDER BY 排序）
SELECT name,
       dept_id,
       salary,
       ROW_NUMBER() OVER (
           PARTITION BY dept_id
           ORDER BY salary DESC
       ) AS rank_in_dept,
       RANK() OVER (ORDER BY salary DESC) AS overall_rank
FROM employees;
```
