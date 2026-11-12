# 业务 Skill 使用记录（skill-usage）

> Skill 1：`shop-trade-check`（自编业务检查 Skill，位于 `shop_word/shop-trade-check/SKILL.md`）
> Skill 2：`shop-trade-acceptance`（自编验收检查 Skill，位于 `shop_word/shop-trade-acceptance/SKILL.md`）
> 约束：只读检查为主，通过演示数据重置保证场景独立，不直接改业务表。

## 1. Skill 定义与检查范围

| 项 | 内容 |
| --- | --- |
| 入口 | `shop_word/shop-trade-check/SKILL.md` + `scripts/acceptance_check.py`；`shop_word/shop-trade-acceptance/SKILL.md` + `scripts/check_evidence.py` |
| 检查维度 | 状态机合法性、库存守恒（初始=可售+占用+出库）、支付幂等、退款与发货互斥、异常只增不删、越权拦截；验收 Skill 另核对 40 项证据覆盖（R/S/E/D/B） |
| 场景 | normal 11 项 / stock 7 项 / refund 10 项 / late 8 项 / races 15 项，共 51 项 |
| 前置 | 服务运行于 `http://127.0.0.1:8099`，`shop.demo-mode=true`（每套件前 `POST /api/demo/reset`） |

## 2. 调用记录

| # | 时间 | Skill | 执行命令 | 环境 | 结果 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-11-11 | check | `python shop-trade-check/scripts/acceptance_check.py`（旧版实现） | 本机 8099 | 51/51 |
| 2 | 2026-11-12 16:19 | check | `py -3 server/shop/scripts/run_acceptance.py --no-build --no-start` | 本机 8099（修复后重建二进制） | **51/51，退出码 0** |
| 3 | 2026-11-12 19:55 | check | 同上（I-18 修复后） | 本机 8099，无 PYTHONIOENCODING 依赖 | **51/51**，日志 `acceptance-20261112-fix2.log` |
| 4 | 2026-11-12 20:30 | check | `py -3 shop-trade-check/scripts/acceptance_check.py --base-url http://127.0.0.1:8099 --config server/shop/docs/acceptance-config.json`（**按 SKILL.md 直接调用**，逐套件 reset） | 本机 8099 | **51/51**（11/7/10/8/15，五套件 exit 0），日志 `acceptance-20261112-skill-call.log` |
| 5 | 2026-11-12 20:25 | acceptance | `py -3 shop-trade-acceptance/scripts/check_evidence.py gin-vue-admin --evidence-dir gin-vue-admin/docs/evidence --output docs/evidence/evidence-check.md`（**首次实际调用**） | 本机 | **43/43 证据覆盖，缺失 0，退出码 0**；索引 `evidence-index.md`，核验输出 `evidence-check.md`；B4（JSONL 导出）在索引中如实标注为环境待办后由人工核对项兜底 |

原始日志：`docs/evidence/acceptance-2026-11-12.log`、`acceptance-20261112-fix.log`、`acceptance-20261112-fix2.log`、`acceptance-20261112-skill-call.log`。

## 2.1 直接调用 vs 包装调用差异（重要经验）

`acceptance_check.py` 单进程跑多套件时**套件间不重置**，前序套件会消耗库存/产生订单导致 races 前置步骤 `KeyError('order_no')`。正确姿势二选一：
1. `run_acceptance.py`（封装了每套件前 `POST /api/demo/reset`）；
2. 直接调用时**逐套件 reset + `--suites <单个套件>`**（本次 #4 采用，留痕于日志）。

此差异为调用方式问题，非产品缺陷；已回写 `.agents/04-deploy-config.md`。

## 3. 交叉约束覆盖（Skill 的核心价值）

| 交叉点 | 检查项示例 | 本次结果 |
| --- | --- | --- |
| 退款 × 发货互斥 | 退款申请中发货被拒；并发退款与发货只产生一种合法终态 | PASS（R5.4 / R5.3） |
| 占用 × 释放守恒 | 下单扣减、取消/超时关闭释放、重复关闭不再释放 | PASS（stock 7/7） |
| 重复通知 × 记款唯一 | 同交易号重复通知只记款一次；交易号不得跨订单 | PASS（R3.2b/R3.3） |
| 迟到支付 × 库存 | 迟到通知不复活订单、不重占库存；异常退款不再恢复库存 | PASS（R4.2/R6.3） |
| 越权 × 角色 | 买家不能查他人订单、不能发货、仓库不能审核退款 | PASS（R8.4/R7.3） |

## 4. 已知限制（如实记录）

1. Skill 依赖 Windows 下的 Python 3.14；控制台默认 GBK，需设置 `PYTHONIOENCODING=utf-8` 才能输出中文。
2. 五个套件必须逐套重置数据；不重置连续执行会因库存合法消耗导致 races 套件下单失败（脚本已内置重置）。
3. Skill 不覆盖性能与压测，只覆盖业务不变量与竞争结果。
