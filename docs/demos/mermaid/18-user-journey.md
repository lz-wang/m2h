---
title: 用户旅程图 (User Journey)
description: Mermaid 用户旅程图语法参考
tags:
  - mermaid
  - user-journey
create_date: 2026-08-29
update_date: 2026-08-29
---

用户旅程图描述不同用户在系统中完成特定任务时所采取的步骤，展示当前工作流并揭示改进空间。

```mermaid
journey
    title My working day
    section Go to work
      Make tea: 5: Me
      Go upstairs: 3: Me
      Do work: 1: Me, Cat
    section Go home
      Go downstairs: 5: Me
      Sit down: 5: Me
```

````markdown
```mermaid
journey
    title My working day
    section Go to work
      Make tea: 5: Me
      Go upstairs: 3: Me
      Do work: 1: Me, Cat
    section Go home
      Go downstairs: 5: Me
      Sit down: 5: Me
```
````

每个用户旅程分为多个**章节（section）**，描述用户尝试完成的任务部分。

任务语法为 `Task name: <score>: <comma separated list of actors>`，其中分数为 1 到 5 之间的整数。

## 参考

- [User Journey - Mermaid](https://mermaid.js.org/syntax/userJourney.html)
