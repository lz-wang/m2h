---
title: Git 配置 (.gitconfig) 语法高亮示例
description: .gitconfig 使用 ini 围栏标签，Chroma 无独立 gitconfig 词法器，按 INI 分节高亮。
tags:
  - 语法高亮
  - 配置文件
create_date: 2026-09-06
update_date: 2026-09-06
---

# Git 配置 (.gitconfig) 语法高亮示例

.gitconfig 使用 ini 围栏标签，Chroma 无独立 gitconfig 词法器，按 INI 分节高亮。

```ini
; .gitconfig 示例文件 - Git 全局配置（INI 风格），用户级设置存于 ~/.gitconfig

[user]
    name = 张三
    email = zhangsan@example.com

[core]
    editor = vim
    autocrlf = input          ; Windows 建议 true，跨平台项目用 input
    quotepath = false         ; 正确显示中文文件名

[init]
    defaultBranch = main

[pull]
    rebase = true             ; 拉取时变基，保持线性提交历史

[push]
    default = current         ; 推送当前分支到同名远程

[alias]
    st = status -sb
    co = checkout
    br = branch
    ci = commit
    lg = log --oneline --graph --decorate -20
    last = log -1 HEAD
    undo = reset --soft HEAD~1
    amend = commit --amend --no-edit

[diff]
    tool = vimdiff

[merge]
    tool = vimdiff
    conflictstyle = diff3     ; 冲突时显示三方（基础/我方/对方）

[color]
    ui = auto

[rerere]
    enabled = true            ; 记忆冲突解决方式，重复变基时自动复用
```
