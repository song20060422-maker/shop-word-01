// 题面 5 个完整工作场景的端到端验收脚本（真实 HTTP + 真实 MySQL + 签名通知）
// 用法：node scripts/shop-acceptance.mjs
// 环境变量：BASE（默认 http://127.0.0.1:8097）、SECRET（默认 channel-shared-secret）、WAIT_SECONDS（默认 70）
import crypto from 'node:crypto';

const BASE = process.env.BASE || 'http://127.0.0.1:8097';
const SECRET = process.env.SECRET || 'channel-shared-secret';
const WAIT = Number(process.env.WAIT_SECONDS || 70);
const P1 = 1; // 商品 A：单价 10000 分，初始可售 5 件

let pass = 0, fail = 0;
const results = [];
function check(name, ok, detail = '') {
  if (ok) { pass++; console.log(`[PASS] ${name}${detail ? ' —— ' + detail : ''}`); }
  else { fail++; console.log(`[FAIL] ${name}${detail ? ' —— ' + detail : ''}`); }
  results.push({ name, ok, detail });
}
const sleep = ms => new Promise(r => setTimeout(r, ms));
const fen = n => `${(n / 100).toFixed(2)} 元`;

async function req(path, { method = 'GET', token, body } = {}) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers.Authorization = 'Bearer ' + token;
  const resp = await fetch(BASE + path, { method, headers, body: body ? JSON.stringify(body) : undefined });
  let json = {};
  try { json = await resp.json(); } catch { /* ignore */ }
  return { status: resp.status, code: json.code, msg: json.msg, data: json.data };
}
async function must(path, opt) {
  const r = await req(path, opt);
  if (r.code !== 0) throw new Error(`${opt?.method || 'GET'} ${path} 失败: code=${r.code} msg=${r.msg}`);
  return r.data;
}
async function login(username) {
  const d = await must('/api/login', { method: 'POST', body: { username, password: 'shop123' } });
  return d.token;
}
function sign(orderNo, txn, amount) {
  return crypto.createHmac('sha256', SECRET).update(`${orderNo}|${txn}|${amount}`).digest('hex');
}
async function notify(orderNo, txn, amount, opt = {}) {
  const sig = opt.badSign ? 'deadbeef' : sign(orderNo, txn, amount);
  return req('/api/pay/notify', { method: 'POST', body: { orderNo, channelTxnId: txn, amountCents: amount, sign: sig } });
}
const reset = () => must('/api/demo/reset', { method: 'POST' });
const summary = token => must('/api/ops/summary', { token });
const productA = async token => (await must('/api/products')).find(p => p.sku === 'SKU-A');

async function main() {
  console.log(`验收目标：${BASE}\n`);

  // ============ 场景 1：活动正常成交 ============
  {
    await reset();
    const ops = await login('ops1'), buyer = await login('buyer1'), wh = await login('wh1');
    await must('/api/ops/products', { method: 'PATCH', token: ops, body: { productId: P1, onSale: false } });
    await must('/api/ops/products', { method: 'PATCH', token: ops, body: { productId: P1, onSale: true } });
    const before = await productA(buyer);
    const created = await must('/api/orders', { method: 'POST', token: buyer, body: { productId: P1, qty: 2 } });
    const orderNo = created.order.orderNo;
    check('1.1 下单快照单价 10000 分且占用库存', created.order.priceCents === 10000 && created.availableAfter === before.available - 2,
      `单价=${created.order.priceCents} 可售=${created.availableAfter}`);

    const n = await notify(orderNo, 'TXN-' + orderNo, created.order.amountCents);
    check('1.2 有效支付通知记款并进入待发货', n.data?.result === 'accepted', `result=${n.data?.result}`);

    const detail = await must(`/api/orders/${orderNo}`, { token: buyer });
    check('1.3 买家可看到收款记录', detail.order.status === 'paid' && !!detail.payment, `status=${detail.order.status}`);

    const shipable = await must('/api/warehouse/shipable', { token: wh });
    check('1.4 仓库可见可发货订单', shipable.some(o => o.orderNo === orderNo), `共 ${shipable.length} 单`);
    await must('/api/warehouse/ship', { method: 'POST', token: wh, body: { orderNo, carrier: '顺丰', trackingNo: 'SF1001' } });

    const afterShip = await must(`/api/orders/${orderNo}`, { token: buyer });
    check('1.5 买家可查询运单号', afterShip.shipment?.trackingNo === 'SF1001', `运单=${afterShip.shipment?.trackingNo}`);

    const sum = await summary(ops);
    const a = sum.products.find(p => p.sku === 'SKU-A');
    check('1.6 运营核查收款与库存', sum.totalPaidCents >= 20000 && a.available === before.available - 2,
      `已收款=${fen(sum.totalPaidCents)} 可售=${a.available}`);
    check('1.7 库存流水可追溯', (await must('/api/ops/ledger', { token: ops })).some(l => l.orderNo === orderNo && l.changeType === 'occupy'));
  }

  // ============ 场景 2：缺货与弃单（20 并发） ============
  {
    await reset();
    const tokens = [await login('buyer1'), await login('buyer2'), await login('buyer3')];
    const jobs = Array.from({ length: 20 }, (_, i) => req('/api/orders', {
      method: 'POST', token: tokens[i % 3], body: { productId: P1, qty: 1 },
    }));
    const rs = await Promise.all(jobs);
    const okList = rs.filter(r => r.code === 0);
    const stockOut = rs.filter(r => r.code !== 0);
    check('2.1 20 并发下单不超卖', okList.length === 5 && stockOut.every(r => r.code === 1003),
      `成功=${okList.length} 失败=${stockOut.length}（库存不足码 1003）`);
    check('2.2 售罄后可售数量为 0', (await productA(tokens[0])).available === 0);

    // 未付款订单关闭释放占用（此处用买家取消；超时关闭在场景 4 验证）
    for (const r of okList) {
      const token = tokens[(rs.indexOf(r)) % 3];
      await must(`/api/orders/${r.data.order.orderNo}/cancel`, { method: 'POST', token });
    }
    check('2.3 关闭未付款订单释放占用', (await productA(tokens[0])).available === 5);

    const created = await must('/api/orders', { method: 'POST', token: tokens[2], body: { productId: P1, qty: 2 } });
    check('2.4 其他买家可购买释放的商品', created.availableAfter === 3, `可售=${created.availableAfter}`);
    await must('/api/orders/' + created.order.orderNo + '/cancel', { method: 'POST', token: tokens[2] });
  }

  // ============ 场景 3：售前退款（首次失败、再次成功） ============
  {
    await reset();
    const buyer = await login('buyer2'), wh = await login('wh1'), cs = await login('cs1');
    const created = await must('/api/orders', { method: 'POST', token: buyer, body: { productId: P1, qty: 2 } });
    const orderNo = created.order.orderNo;
    await notify(orderNo, 'TXN-' + orderNo, created.order.amountCents);

    const applied = await must(`/api/orders/${orderNo}/refund`, { method: 'POST', token: buyer, body: { orderNo, reason: '不想要了' } });
    check('3.1 买家可申请整单退款', applied.refundPending === true, `退款单=${applied.refund.refundNo}`);

    const blocked = await req('/api/warehouse/ship', { method: 'POST', token: wh, body: { orderNo, carrier: '顺丰', trackingNo: 'SF-X' } });
    check('3.2 退款申请期间发货受限', blocked.status === 409, `HTTP ${blocked.status} ${blocked.msg}`);

    await must('/api/cs/channel/mode', { method: 'POST', token: cs, body: { refundMode: 'fail' } });
    const first = await req('/api/cs/refunds/audit', { method: 'POST', token: cs, body: { orderNo, approve: true, reason: '同意整单退款' } });
    check('3.3 首次渠道退款失败并保留记录', first.status === 409 && first.data?.channelStatus === 'failed',
      `code=${first.code} channel=${first.data?.channelStatus}`);

    await must('/api/cs/channel/mode', { method: 'POST', token: cs, body: { refundMode: 'success' } });
    const retry = await must('/api/cs/refunds/retry', { method: 'POST', token: cs, body: { refundNo: applied.refund.refundNo } });
    check('3.4 客服重试后渠道退款成功', retry.channelStatus === 'success' && retry.auditStatus === 'approved');

    const detail = await must(`/api/orders/${orderNo}`, { token: buyer });
    check('3.5 买家可见退款结果且订单已退款', detail.order.status === 'refunded' && detail.refund?.channelStatus === 'success');
    check('3.6 售前退款恢复可售数量', (await productA(buyer)).available === 5, `可售=${(await productA(buyer)).available}`);

    const rejected = await req('/api/warehouse/ship', { method: 'POST', token: wh, body: { orderNo, carrier: '顺丰', trackingNo: 'SF-Y' } });
    check('3.7 已退款订单不能发货', rejected.status === 409, `HTTP ${rejected.status} ${rejected.msg}`);
  }

  // ============ 场景 4：迟到支付（超时关闭后通知到达） ============
  {
    await reset();
    const buyer = await login('buyer3'), cs = await login('cs1'), ops = await login('ops1');
    const created = await must('/api/orders', { method: 'POST', token: buyer, body: { productId: P1, qty: 2 } });
    const orderNo = created.order.orderNo;
    console.log(`\n… 等待 ${WAIT}s 让订单按服务端时间超时（演示配置 1 分钟保留期）`);
    await sleep(WAIT * 1000);
    const closed = await must('/api/jobs/close-expired', { method: 'POST' });
    const afterClose = await productA(buyer);
    check('4.1 超时关闭并释放占用', closed.closed >= 1 && afterClose.available === 5, `关闭=${closed.closed} 可售=${afterClose.available}`);

    const n = await notify(orderNo, 'TXN-LATE-' + orderNo, created.order.amountCents);
    const d1 = await must(`/api/orders/${orderNo}`, { token: buyer });
    check('4.2 迟到支付不恢复订单', n.data?.result === 'abnormal' && d1.order.status === 'closed', `result=${n.data?.result} status=${d1.order.status}`);
    check('4.3 迟到支付不重新占用库存', (await productA(buyer)).available === 5);

    const abns = await must('/api/cs/abnormals', { token: cs });
    const target = abns.find(a => a.orderNo === orderNo && a.status === 'pending');
    check('4.4 客服从异常列表发现记录', !!target, `异常单=${target?.channelTxnId}`);
    if (!target) {
      console.log('未产生异常记录（订单可能尚未超时），跳过 4.5~4.8');
      console.log(`\n验收结果：PASS ${pass}，FAIL ${fail}`);
      process.exit(fail > 0 ? 1 : 0);
    }

    const handled = await must('/api/cs/abnormals/handle', { method: 'POST', token: cs, body: { channelTxnId: target.channelTxnId, reason: '核实为迟到支付，发起退款' } });
    check('4.5 客服发起退款成功', handled.channelStatus === 'success', `渠道=${handled.channelStatus}`);
    check('4.6 迟到支付退款不重复恢复库存', (await productA(buyer)).available === 5);

    const sum = await summary(ops);
    check('4.7 运营确认无重复释放与重复退款', sum.totalPaidCents === 20000 && sum.totalRefundedCents === 20000 && sum.products.find(p => p.sku === 'SKU-A').available === 5,
      `已收款=${fen(sum.totalPaidCents)} 已退款=${fen(sum.totalRefundedCents)}`);

    const again = await notify(orderNo, 'TXN-LATE-' + orderNo, created.order.amountCents);
    const abns2 = await must('/api/cs/abnormals', { token: cs });
    check('4.8 重复迟到通知不产生第二条异常记录', abns2.filter(a => a.orderNo === orderNo).length === 1, `result=${again.data?.result} 条数=${abns2.filter(a => a.orderNo === orderNo).length}`);
  }

  // ============ 场景 5：操作竞争与越权 ============
  {
    await reset();
    const b1 = await login('buyer1'), b2 = await login('buyer2'), wh = await login('wh1'), cs = await login('cs1');
    check('5.1 买家登录成功', !!b1 && !!b2);

    const o2 = await must('/api/orders', { method: 'POST', token: b2, body: { productId: P1, qty: 1 } });
    const other = await req(`/api/orders/${o2.order.orderNo}`, { token: b1 });
    check('5.2 越权查看他人订单被拒', other.status === 403, `code=${other.code} ${other.msg}`);
    const cancelOther = await req(`/api/orders/${o2.order.orderNo}/cancel`, { method: 'POST', token: b1 });
    check('5.3 越权取消他人订单被拒', cancelOther.status === 403, `code=${cancelOther.code} ${cancelOther.msg}`);
    const crossRole = await req('/api/warehouse/shipable', { token: b1 });
    check('5.4 买家调用仓库接口被拒', crossRole.status === 403, `HTTP ${crossRole.status}`);
    const anon = await req('/api/ops/summary');
    check('5.5 未登录访问被拒', anon.status === 401, `HTTP ${anon.status}`);

    // 重复支付通知幂等
    const paid = await must('/api/orders', { method: 'POST', token: b1, body: { productId: P1, qty: 1 } });
    const txn = 'TXN-DUP-' + paid.order.orderNo;
    const first = await notify(paid.order.orderNo, txn, paid.order.amountCents);
    const dup = await notify(paid.order.orderNo, txn, paid.order.amountCents);
    const pd = await must(`/api/orders/${paid.order.orderNo}`, { token: b1 });
    check('5.6 重复支付通知不重复记款', first.data?.result === 'accepted' && dup.data?.result === 'duplicated' && pd.order.status === 'paid',
      `first=${first.data?.result} dup=${dup.data?.result}`);

    // 退款申请与发货并发：只能一种合法结果
    const race = await must('/api/orders', { method: 'POST', token: b2, body: { productId: P1, qty: 2 } });
    await notify(race.order.orderNo, 'TXN-RACE-' + race.order.orderNo, race.order.amountCents);
    const [shipR, refundR] = await Promise.all([
      req('/api/warehouse/ship', { method: 'POST', token: wh, body: { orderNo: race.order.orderNo, carrier: '顺丰', trackingNo: 'SF-RACE' } }),
      req(`/api/orders/${race.order.orderNo}/refund`, { method: 'POST', token: b2, body: { orderNo: race.order.orderNo, reason: '并发退款' } }),
    ]);
    const shipOK = shipR.code === 0, refundOK = refundR.code === 0;
    const rd = await must(`/api/orders/${race.order.orderNo}`, { token: b2 });
    const legal = shipOK !== refundOK && ((shipOK && rd.order.status === 'shipped') || (refundOK && rd.order.status === 'refund_requested'));
    check('5.7 退款与发货并发只形成一种结果', legal, `ship=${shipOK} refund=${refundOK} status=${rd.order.status}`);

    // 伪造签名留痕
    const bad = await notify(paid.order.orderNo, 'TXN-FAKE', paid.order.amountCents, { badSign: true });
    check('5.8 验签失败可查询处理结果', bad.code === 3001, `code=${bad.code} ${bad.msg}`);

    // 客服审核越权（买家不能审核）
    const audit = await req('/api/cs/refunds/audit', { method: 'POST', token: b1, body: { orderNo: race.order.orderNo, approve: true } });
    check('5.9 买家不能调用客服审核接口', audit.status === 403, `HTTP ${audit.status}`);
    void cs;
  }

  console.log('\n----------------------------------------');
  console.log(`验收结果：PASS ${pass}，FAIL ${fail}`);
  if (fail > 0) process.exit(1);
}

main().catch(err => { console.error('验收脚本异常：', err.message); process.exit(1); });
