"""Shop 商城交易系统 · 一键验收脚本

作用：
  1. 编译并启动 GVA 服务端（可选 --no-build / --no-start 接管已有进程）
  2. 每个验收套件执行前调用 POST /api/demo/reset 重置演示数据（保证各场景从
     「SKU-A 可售 5 件、SKU-B 可售 3 件、无历史订单」的干净现场起跑）
  3. 依次执行 normal / stock / refund / late / races 五个套件并汇总结果

用法：
  python scripts/run_acceptance.py                       # 全流程
  python scripts/run_acceptance.py --no-build --no-start  # 服务已在跑，只跑验收
  python scripts/run_acceptance.py --acceptance-dir D:\\path\\shop-trade-check
"""

import argparse
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
SHOP_DIR = os.path.dirname(HERE)
SERVER_DIR = os.path.dirname(SHOP_DIR)
CONFIG_YAML = os.path.join(SERVER_DIR, "config.yaml")
DEFAULT_ACCEPTANCE_DIR = r"D:\Users\Lenovo\Desktop\shop_word\shop-trade-check"
SUITES = ["normal", "stock", "refund", "late", "races"]


def read_base_url() -> str:
    """从 config.yaml 的 shop 段解析端口，默认 8099。"""
    port = 8099
    in_shop = False
    try:
        for line in open(CONFIG_YAML, encoding="utf-8"):
            low = line.strip()
            if low and not low.startswith("#") and not low.startswith(" "):
                in_shop = low.startswith("shop:")
                continue
            if not in_shop:
                continue
            if low.startswith("addr:") or low.startswith("http-port:"):
                port = int(low.split(":")[1].strip().strip('"'))
    except Exception:
        pass
    return "http://127.0.0.1:%d" % port


def http(method, url, body=None, token=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            return r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()


def build_and_start(exe: str, log: str) -> bool:
    subprocess.run(["powershell", "-NoProfile", "-Command",
                    "Get-Process gva-shop -ErrorAction SilentlyContinue | Stop-Process -Force"],
                   capture_output=True)
    time.sleep(2)
    r = subprocess.run(["go", "build", "-o", exe, "./"], cwd=SERVER_DIR,
                       capture_output=True, text=True, encoding="utf-8", errors="replace")
    if r.returncode != 0:
        print(r.stderr[-2000:])
        return False
    subprocess.Popen([exe], cwd=SERVER_DIR,
                     stdout=open(log, "w", encoding="utf-8", errors="replace"),
                     stderr=subprocess.STDOUT,
                     creationflags=0x00000008 | 0x00000200)
    return True


def wait_health(base: str, seconds: int = 40) -> bool:
    for _ in range(seconds):
        time.sleep(1)
        try:
            code, body = http("GET", base + "/api/health")
            if code == 200 and "ok" in body:
                return True
        except Exception:
            pass
    return False


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--no-build", action="store_true", help="不重新编译")
    ap.add_argument("--no-start", action="store_true", help="不重启服务（服务已在运行）")
    ap.add_argument("--acceptance-dir", default=DEFAULT_ACCEPTANCE_DIR)
    ap.add_argument("--suites", default=",".join(SUITES))
    args = ap.parse_args()

    base = read_base_url()
    exe = os.path.join(os.environ.get("TEMP", "."), "gva-shop.exe")
    log = os.path.join(os.environ.get("TEMP", "."), "gva-shop.log")

    if not args.no_start:
        if not args.no_build and not build_and_start(exe, log):
            print("编译失败，请先修复后再验收")
            return 2
        if not wait_health(base):
            print("服务未就绪：%s/api/health" % base)
            return 2
    print("服务地址：%s" % base)

    script = os.path.join(args.acceptance_dir, "scripts", "acceptance_check.py")
    cfg = os.path.join(SHOP_DIR, "docs", "acceptance-config.json")
    if not os.path.exists(script):
        print("未找到验收脚本：%s" % script)
        return 2

    total_pass = total = 0
    failed = []
    for suite in [s for s in args.suites.split(",") if s.strip()]:
        code, body = http("POST", base + "/api/demo/reset", {})
        if code != 200:
            print("[%s] 演示数据重置失败：%s %s" % (suite, code, body[:200]))
            failed.append(suite)
            continue
        print("\n== 套件 %s（已重置演示数据）==" % suite)
        r = subprocess.run([sys.executable, script, "--base-url", base,
                            "--config", cfg, "--suites", suite],
                           capture_output=True, text=True, encoding="utf-8", errors="replace")
        out = r.stdout
        print(out.strip())
        line = next((l for l in out.splitlines() if l.startswith("结果")), "")
        if line:
            p, m = line.split(":")[1].split("/")[0].strip(), line.split("/")[1].split(" ")[0].strip()
            total_pass += int(p)
            total += int(m)
        if r.returncode != 0:
            failed.append(suite)

    print("\n========== 汇总 ==========")
    print("检查项通过：%d/%d" % (total_pass, total))
    if failed:
        print("未通过套件：%s" % ",".join(failed))
        return 1
    print("全部套件通过")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
