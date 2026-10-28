#!/usr/bin/env python3
"""Run an isolated Nginx/m2h subpath smoke; requires Python 3 and nginx."""

import argparse
import contextlib
import http.client
import json
import shutil
import socket
import subprocess
import tempfile
import time
from pathlib import Path
from urllib.parse import quote, urlsplit


def free_port():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


def request(port, path, method="GET"):
    connection = http.client.HTTPConnection("127.0.0.1", port, timeout=3)
    try:
        connection.request(method, path)
        response = connection.getresponse()
        return response.status, dict(response.getheaders()), response.read()
    finally:
        connection.close()


def ready(process, port, path):
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError(f"process exited during startup: {process.returncode}")
        try:
            if request(port, path)[0] == 200:
                return
        except OSError:
            pass
        time.sleep(0.1)
    raise RuntimeError(f"service did not become ready at port {port}{path}")


@contextlib.contextmanager
def running(command, log):
    with log.open("wb") as output:
        process = subprocess.Popen(command, stdout=output, stderr=subprocess.STDOUT)
        try:
            yield process
        except Exception:
            output.flush()
            print(log.read_text(errors="replace"))
            raise
        finally:
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=3)


def check(binary, nginx, base):
    with tempfile.TemporaryDirectory(prefix="m2h-nginx-") as temporary:
        root = Path(temporary)
        docs = root / "docs"
        docs.mkdir()
        (docs / "README.md").write_text("# Proxy\n\n![Encoded](%E4%B8%AD%20%25.png)\n")
        (docs / "中 %.png").write_bytes(b"encoded attachment")
        (docs / "bad.html").write_text("<script>bad</script>")
        upstream, proxy = free_port(), free_port()
        while proxy == upstream:
            proxy = free_port()
        configuration = root / "nginx.conf"
        configuration.write_text(f"""
pid "{root}/nginx.pid";
error_log stderr;
events {{ worker_connections 32; }}
http {{
    access_log off;
    client_body_temp_path "{root}/client_body";
    proxy_temp_path "{root}/proxy";
    fastcgi_temp_path "{root}/fastcgi";
    uwsgi_temp_path "{root}/uwsgi";
    scgi_temp_path "{root}/scgi";
    server {{
        listen 127.0.0.1:{proxy};
        location = {base} {{ return 308 {base}/$is_args$args; }}
        location ^~ {base}/ {{
            proxy_pass http://127.0.0.1:{upstream};
            proxy_http_version 1.1;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }}
        location / {{ return 404; }}
    }}
}}
""")
        with running([str(binary), str(docs), "--no-open", "--port", str(upstream),
                      "--base-path", base], root / "m2h.log") as backend:
            ready(backend, upstream, f"{base}/healthz")
            with running([nginx, "-p", str(root), "-c", str(configuration),
                          "-g", "daemon off;"], root / "nginx.log") as frontend:
                ready(frontend, proxy, f"{base}/healthz")
                for route in ["/", "/doc/README.md", "/api/files",
                              "/api/document?path=README.md", "/raw/README.md",
                              "/ui/favicon.svg", "/runtime/katex.min.js"]:
                    status, headers, body = request(proxy, base + route)
                    assert status == 200, (base, route, status, body[:200])
                    assert "Content-Security-Policy" in headers, (base, route, headers)
                status, headers, body = request(proxy, f"{base}/api/document?path=README.md")
                assert f"{base}/assets/%E4%B8%AD%20%25.png" in json.loads(body)["html"]
                assert request(proxy, f"{base}/assets/{quote('中 %.png')}")[2] == b"encoded attachment"
                get = request(proxy, f"{base}/doc/README.md")
                head = request(proxy, f"{base}/doc/README.md", "HEAD")
                assert get[0] == head[0] == 200 and head[2] == b""
                for header in ["Content-Type", "Cache-Control", "Content-Security-Policy"]:
                    assert get[1][header] == head[1][header], (header, get[1], head[1])
                for route, expected in [
                    ("?mode=dark", f"{base}/?mode=dark"),
                    ("/ui?x=1", f"{base}/ui/?x=1"),
                    ("/runtime", f"{base}/runtime/"),
                    ("/doc/../assets/bad.html", f"{base}/assets/bad.html"),
                ]:
                    status, headers, _ = request(proxy, base + route)
                    assert 300 <= status < 400, (route, status)
                    location = urlsplit(headers["Location"])
                    target = location.path + ("?" + location.query if location.query else "")
                    assert target == expected, (route, target, expected)
                assert request(proxy, f"{base}/assets/bad.html")[0] == 404
                assert request(proxy, "/api/files")[0] == 404
                assert request(proxy, base + "-other/")[0] == 404
        print(f"[nginx-smoke] {base}: URI, API, HEAD, redirects and encoded attachments passed")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("--nginx", default="nginx")
    args = parser.parse_args()
    binary = args.binary.resolve()
    if not binary.is_file():
        parser.error(f"binary does not exist: {binary}")
    nginx = shutil.which(args.nginx)
    if nginx is None:
        parser.error("nginx is required; install it or pass --nginx /path/to/nginx")
    for base in ["/docs", "/apps/docs"]:
        check(binary, nginx, base)


if __name__ == "__main__":
    main()
