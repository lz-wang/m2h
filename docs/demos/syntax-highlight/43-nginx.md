---
title: Nginx 语法高亮示例
description: 以 nginx 围栏代码块渲染 nginx.conf，验证 Nginx 的语法高亮效果。
tags:
  - 语法高亮
  - 配置文件
create_date: 2026-09-06
update_date: 2026-09-06
---

# Nginx 语法高亮示例

以下 `nginx.conf` 示例使用 ` ```nginx ` 围栏代码块渲染：

```nginx
# nginx.conf 示例文件 - Nginx 主配置，events/http/server/location 分层结构

# ================ 全局指令 ================
user  nginx;
worker_processes  auto;
error_log  /var/log/nginx/error.log warn;
pid        /var/run/nginx.pid;

events {
    worker_connections  1024;        # 每个 worker 进程最大连接数
}

http {
    include       /etc/nginx/mime.types;
    default_type  application/octet-stream;

    sendfile        on;
    keepalive_timeout  65;

    # Gzip 压缩
    gzip  on;
    gzip_types text/plain application/json application/javascript text/css;

    # ================ upstream 上游服务器组（负载均衡）================
    upstream backend {
        server 127.0.0.1:8000 weight=3;
        server 127.0.0.1:8001 weight=1;
    }

    # ================ server 虚拟主机 ================
    server {
        listen       80;
        server_name  example.com www.example.com;

        # location 匹配优先级：= > ^~ > ~ / ~* > 普通前缀
        # =    精确匹配（最高优先级）
        # ^~   前缀匹配命中后不再进入正则
        # ~    区分大小写的正则；~* 不区分大小写

        # 精确匹配
        location = /favicon.ico {
            return 204;
        }

        # 静态文件根目录
        location / {
            root   /usr/share/nginx/html;
            index  index.html;
            try_files $uri $uri/ =404;
        }

        # 反向代理到上游
        location /api/ {
            proxy_pass http://backend;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        }

        # 正则匹配（静态资源缓存）
        location ~* \.(jpg|png|gif|css|js)$ {
            expires 30d;
        }
    }
}
```
