# 08 · 业务流实现模式（写 service 代码时必读）

## 模式 A：状态跃迁 = 条件 UPDATE + 行数校验

```go
res := tx.Model(&model.Order{}).
    Where("order_no = ? AND status = ? AND expire_at > ?", orderNo, model.OrderStatusPendingPayment, now).
    Updates(map[string]interface{}{"status": ..., "updated_at": now})
if res.Error != nil { return res.Error }
if res.RowsAffected != 1 { /* 读最新态，区分重复/过期/非法，给对应业务错误 */ }
```

禁止：先 SELECT 判断再 UPDATE（并发窗口）；handler 内改状态。

## 模式 B：幂等 = 数据库唯一键 + 冲突分类

- 幂等键落唯一索引（`payments.channel_txn_id`、`payments.order_no`、`shipments.order_no`）。
- 插入失败用 `isDuplicateEntry`（errors.As + errno 1062）判定重复；其它 DB 错误原样上抛。

## 模式 C：一次性资源闸门（库存恢复/渠道调用）

- 库存恢复：`restored` 布尔闸门（`UPDATE ... WHERE restored = false`，RowsAffected==1 才释放）。
- 渠道调用：事务内 `clause.Locking{Strength:"UPDATE"}` 行锁读单 → 校验入口状态 → 锁内调渠道（模拟渠道为同步内存实现）→ 锁内推进。后到者被闸门拒绝（`ErrRefundNotRetryable`）。
- 迟到支付退款：**绝不**走库存恢复路径。

## 模式 D：审计留痕

每次状态跃迁调 `logOp(tx, orderNo, actorID, role, action, detail)`，与业务同事务提交，禁止事后补写。

## 模式 E：可复现演示开关

- 渠道退款结果可切 `success|fail`（`SimulatedChannel.SetRefundMode`），并带 `RefundCalls()` 计数供并发测试断言。
- 超时可缩短但判断必须用服务端时间；演示值在文档注明。

## 反模式（出现过即打回）

1. 在 handler/service 混层写事务。
2. 用 panic/忽略错误代替业务错误码。
3. 用内存 map 冒充持久化状态。
4. 为凑覆盖率给工具函数写用例而业务分支裸奔。
