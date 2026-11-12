# 业务 Skill 使用记录（skill-usage）

> Skill：`shop-trade-check`（自编业务检查 Skill，位于 `shop_word/shop-trade-check/SKILL.md`）
> 用途：在真实 HTTP 环境检查订单、库存、支付、退款、发货之间的业务约束及跨环节异常。
> 约束：只读检查为主，通过演示数据重置保证场景独立，不直接改业务表。

## 1. Skill 定义与检查范围

| 项 | 内容 |
| --- | --- |
| 入口 | `shop_word/shop-trade-check/SKILL.md` + `scripts/acceptance_check.py` |
| 检查维度 | 状态机合法性、库存守恒（初始=可售+占用+出库）、支付幂等、退款与发货互斥、异常只增不删、越权拦截 |
| 场景 | normal 11 项 / stock 7 项 / refund 10 项 / late 8 项 / races 15 项，共 51 项 |
| 前置 | 服务运行于 `http://127.0.0.1:8099`，`shop.demo-mode=true`（每套件前 `POST /api/demo/reset`） |

## 2. 调用记录

| # | 时间 | 执行命令 | 环境 | 结果 |
| --- | --- | --- | --- | --- |
| 1 | 2026-11-11 | `python shop-trade-check/scripts/acceptance_check.py`（旧版实现） | 本机 8099 | 51/51 |
| 2 | 2026-11-12 16:19 | `py -3 server/shop/scripts/run_acceptance.py --no-build --no-start` | 本机 8099（本次修复后重建二进制） | **51/51，退出码 0** |

原始日志：`docs/evidence/acceptance-2026-11-12.log`。

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
