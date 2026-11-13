import service from '@/utils/request'
// Shop 商城后台管理（运营/仓库/客服）接口封装
// 后端：server/shop/adminapi（挂在 GVA PrivateGroup，走 gva 登录 + Casbin）

// ---------- 运营 ----------
export const opsSummary = () => service({ url: '/shop/ops/summary', method: 'get' })
export const opsProducts = () => service({ url: '/shop/ops/products', method: 'get' })
export const opsUpdateProduct = (id, data) => service({ url: `/shop/ops/products/${id}`, method: 'patch', data })
export const opsOrders = (params) => service({ url: '/shop/ops/orders', method: 'get', params })
export const opsOrderTrace = (orderNo) => service({ url: `/shop/ops/orders/${orderNo}`, method: 'get' })
export const opsNotices = (params) => service({ url: '/shop/ops/notices', method: 'get', params })
export const opsAbnormals = (params) => service({ url: '/shop/ops/abnormals', method: 'get', params })
export const opsLedger = (params) => service({ url: '/shop/ops/ledger', method: 'get', params })
export const opsTrail = (params) => service({ url: '/shop/ops/trail', method: 'get', params })

// ---------- 仓库 ----------
export const whShipable = () => service({ url: '/shop/warehouse/shipable', method: 'get' })
export const whShip = (data) => service({ url: '/shop/warehouse/ship', method: 'post', data })
export const whShipments = () => service({ url: '/shop/warehouse/shipments', method: 'get' })
export const whOrderTrace = (orderNo) => service({ url: `/shop/warehouse/orders/${orderNo}`, method: 'get' })

// ---------- 客服 ----------
export const csRefunds = (params) => service({ url: '/shop/cs/refunds', method: 'get', params })
export const csRefundAudit = (data) => service({ url: '/shop/cs/refunds/audit', method: 'post', data })
export const csRefundRetry = (data) => service({ url: '/shop/cs/refunds/retry', method: 'post', data })
export const csAbnormals = (params) => service({ url: '/shop/cs/abnormals', method: 'get', params })
export const csHandleAbnormal = (data) => service({ url: '/shop/cs/abnormals/handle', method: 'post', data })
export const csNotices = (params) => service({ url: '/shop/cs/notices', method: 'get', params })
export const csOrderTrace = (orderNo) => service({ url: `/shop/cs/orders/${orderNo}`, method: 'get' })
export const csChannelMode = (data) => service({ url: '/shop/cs/channel/mode', method: 'post', data })
