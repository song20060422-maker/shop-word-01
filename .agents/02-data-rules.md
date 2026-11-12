# 02 · 数据与状态机规则

## 状态机

```
订单：pending_payment → paid → shipped
        │  ↑(审核拒绝)
        ├→ cancelled / closed(超时)
        └→ refund_requested → refunded(渠道成功) / refund_failed(渠道失败,可重试)
```

- 所有跃迁实现位置：`service/order.go`、`service/payment.go`、`service/refund.go`、`service/shipment.go`。
- 完成判据必须可观察（状态、数字、记录、错误码），不接受"已实现"。

## 业务规则矩阵（BR-01~12 摘要）

| BR | 要点 | 落点 |
|---|---|---|
| BR-01/02 | 条件 UPDATE 扣减 + 快照 + 占用流水 | `order.go Create` |
| BR-03 | 10 分钟超时（服务端时间）、释放幂等 | `Cancel/CloseExpired` |
| BR-04~06 | HMAC 验签 → 金额 → 幂等（唯一键），拒绝可查 | `payment.go` |
| BR-07 | 有效付款推进 paid；迟到转 abnormal_payments | `payment.go Notify` |
| BR-08~10 | 退款申请互斥发货；渠道确认才 refunded；迟到退款不恢复库存 | `refund.go` |
| BR-11 | 发货唯一（shipments 唯一键） | `shipment.go Ship` |
| BR-12 | 核查汇总=明细之和；异常无"改单消失"通路 | `report.go` |

完整判据见 `shop_word/references/business-rules.md`（只读）。

## 异常矩阵（E-01~12）

并发争抢、取消/超时竞争、重复通知、验签失败、迟到支付、退款×发货竞争、渠道失败重试、已发货退款、越权、重复发货、改单消除异常——全部有明确拒绝/幂等/互斥行为，见 `versions/dev-agent.v3.md` §三。

## 不变量（每次改动后复核）

| ID | 不变量 | 复核方式 |
|---|---|---|
| INV-1 | 初始可售 = 可售 + 各态占用 + 已出库 | 商品与订单聚合对账 |
| INV-2 | 同一渠道交易号至多归属一个订单 | `payments/abnormal_payments` 查询 |
| INV-3 | 每单至多一次成功发货 | `shipments` |
| INV-4 | 每单至多一次成功退款 | `refunds` |

## 关键防御（审计 I-18 整改后成为固定约束）

1. **渠道调用闸门**：`Retry`/`Audit(approve)`/`HandleAbnormal` 在事务内 `SELECT ... FOR UPDATE` 行锁 + 入口状态校验后才调渠道；并发后到者被闸门拒绝。模拟渠道同步内存实现可事务内调用；真实渠道改闸门状态机+异步回调。
2. **错误分类**：唯一键冲突（errno 1062，`isDuplicateEntry`）才判"重复"，其它 DB 错误原样上抛，禁止一律映射为业务重复错误。
3. **RowsAffected 校验**：条件更新推进状态必须校验受影响行数；不为 1 即回滚。
