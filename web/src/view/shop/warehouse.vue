<template>
  <div class="shop-page">
    <el-card shadow="never" class="mb8">
      <template #header><span>可发货订单（仅已付款且无进行中退款；一单只能登记一次）</span></template>
      <el-table :data="shipable" size="small">
        <el-table-column prop="orderNo" label="订单号" width="190" />
        <el-table-column prop="productName" label="商品" min-width="120" />
        <el-table-column prop="qty" label="数量" width="60" />
        <el-table-column label="金额" width="90"><template #default="{ row }">{{ fen(row.amountCents) }}</template></el-table-column>
        <el-table-column label="付款时间" width="160"><template #default="{ row }">{{ fmt(row.paidAt) }}</template></el-table-column>
        <el-table-column label="承运方 / 运单号" min-width="220">
          <template #default="{ row }">
            <el-input v-model="row._carrier" size="small" placeholder="承运方" style="width: 100px" />
            <el-input v-model="row._trackingNo" size="small" placeholder="运单号" style="width: 120px" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-button size="small" type="primary" @click="ship(row)">登记发货</el-button>
            <el-button size="small" @click="openTrace(row.orderNo)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card shadow="never">
      <template #header><span>发货记录</span></template>
      <el-table :data="shipments" size="small">
        <el-table-column prop="orderNo" label="订单号" width="190" />
        <el-table-column prop="carrier" label="承运方" width="140" />
        <el-table-column prop="trackingNo" label="运单号" min-width="140" />
        <el-table-column label="登记时间" width="160"><template #default="{ row }">{{ fmt(row.createdAt) }}</template></el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="traceVisible" title="订单详情" width="680px">
      <el-descriptions v-if="traceData" :column="2" size="small" border>
        <el-descriptions-item label="订单号">{{ traceData.order.orderNo }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ traceData.order.status }}</el-descriptions-item>
        <el-descriptions-item label="商品">{{ traceData.order.productName }} × {{ traceData.order.qty }}</el-descriptions-item>
        <el-descriptions-item label="金额">{{ fen(traceData.order.amountCents) }}</el-descriptions-item>
        <el-descriptions-item label="可发货">{{ traceData.shipableNow ? '是' : '否' }}</el-descriptions-item>
        <el-descriptions-item v-if="traceData.shipment" label="已发货">{{ traceData.shipment.carrier }} / {{ traceData.shipment.trackingNo }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </div>
</template>

<script setup name="ShopWarehouse">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { whShipable, whShip, whShipments, whOrderTrace } from '@/api/shop'

const shipable = ref([])
const shipments = ref([])
const traceData = ref(null)
const traceVisible = ref(false)

const fen = (v) => ((v || 0) / 100).toFixed(2)
const fmt = (t) => (t ? new Date(t).toLocaleString() : '-')

const load = async () => {
  const [a, s] = await Promise.all([whShipable(), whShipments()])
  shipable.value = (a || []).map(x => ({ ...x, _carrier: '', _trackingNo: '' }))
  shipments.value = s || []
}
const ship = async (row) => {
  if (!row._carrier || !row._trackingNo) {
    ElMessage.warning('请填写承运方与运单号')
    return
  }
  await whShip({ orderNo: row.orderNo, carrier: row._carrier, trackingNo: row._trackingNo })
  ElMessage.success('发货登记成功')
  await load()
}
const openTrace = async (orderNo) => {
  traceData.value = await whOrderTrace(orderNo)
  traceVisible.value = true
}

onMounted(load)
</script>

<style scoped>
.mb8 { margin-bottom: 8px; }
</style>
