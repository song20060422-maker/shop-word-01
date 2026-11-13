<template>
  <div class="shop-page">
    <el-card shadow="never" class="mb8">
      <template #header>
        <div class="flex-between">
          <span>退款审核（同意后调渠道；失败保留记录可重试；申请期间发货暂停）</span>
          <div>
            <el-button size="small" type="danger" @click="setMode('fail')">首次失败（fail）</el-button>
            <el-button size="small" type="success" @click="setMode('success')">退款成功（success）</el-button>
            <span class="muted">当前：{{ channel.refundMode }} · 累计 {{ channel.refundCalls || 0 }} 次</span>
          </div>
        </div>
      </template>
      <el-table :data="refunds" size="small">
        <el-table-column prop="refundNo" label="退款单" width="190" />
        <el-table-column prop="orderNo" label="订单" width="190" />
        <el-table-column prop="source" label="来源" width="90" />
        <el-table-column label="金额" width="90"><template #default="{ row }">{{ fen(row.amountCents) }}</template></el-table-column>
        <el-table-column label="审核" width="90">
          <template #default="{ row }"><el-tag size="small" :type="tagType(row.auditStatus)">{{ row.auditStatus }}</el-tag></template>
        </el-table-column>
        <el-table-column label="渠道" width="90">
          <template #default="{ row }"><el-tag size="small" :type="tagType(row.channelStatus)">{{ row.channelStatus }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="retryCount" label="重试" width="60" />
        <el-table-column prop="auditReason" label="原因" min-width="120" />
        <el-table-column label="操作" width="170">
          <template #default="{ row }">
            <el-button v-if="row.auditStatus === 'pending'" size="small" type="success" @click="audit(row, true)">同意</el-button>
            <el-button v-if="row.auditStatus === 'pending'" size="small" type="danger" @click="audit(row, false)">拒绝</el-button>
            <el-button v-if="row.auditStatus === 'approved' && row.channelStatus === 'failed'" size="small" type="warning" @click="retry(row)">重试退款</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-row :gutter="12">
      <el-col :span="12">
        <el-card shadow="never" class="mb8">
          <template #header><span>异常收款（迟到支付，核实后发起退款）</span></template>
          <el-table :data="abnormals" size="small">
            <el-table-column prop="channelTxnId" label="交易号" width="170" />
            <el-table-column prop="orderNo" label="订单" width="180" />
            <el-table-column label="金额" width="80"><template #default="{ row }">{{ fen(row.amountCents) }}</template></el-table-column>
            <el-table-column prop="status" label="状态" width="90" />
            <el-table-column prop="reason" label="原因" min-width="110" />
            <el-table-column label="操作" width="110">
              <template #default="{ row }">
                <el-button v-if="row.status === 'pending'" size="small" type="warning" @click="handleAbn(row)">发起退款</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="never" class="mb8">
          <template #header><span>支付通知处理记录（只增不改）</span></template>
          <el-table :data="notices" size="small" max-height="360">
            <el-table-column prop="orderNo" label="订单" width="180" />
            <el-table-column prop="channelTxnId" label="交易号" width="160" />
            <el-table-column label="验签" width="60"><template #default="{ row }">{{ row.signOk ? '通过' : '失败' }}</template></el-table-column>
            <el-table-column prop="result" label="结果" width="100" />
            <el-table-column prop="reason" label="原因" min-width="110" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup name="ShopCs">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  csRefunds, csRefundAudit, csRefundRetry, csAbnormals, csHandleAbnormal, csNotices, csChannelMode
} from '@/api/shop'

const refunds = ref([])
const abnormals = ref([])
const notices = ref([])
const channel = ref({})

const fen = (v) => ((v || 0) / 100).toFixed(2)
const tagType = (s) => (['success', 'approved', 'refunded', 'handled', 'accepted'].includes(s) ? 'success'
  : (['failed', 'rejected', 'abnormal'].includes(s) ? 'danger' : 'warning'))

const load = async () => {
  const [r, a, n] = await Promise.all([csRefunds({}), csAbnormals({}), csNotices({ limit: 50 })])
  refunds.value = r || []
  abnormals.value = a || []
  notices.value = (n || []).slice(0, 50)
}
const audit = async (row, approve) => {
  const { value } = await ElMessageBox.prompt(approve ? '同意备注：' : '拒绝原因：', '退款审核', {
    inputValue: approve ? '同意整单退款' : '发货在即，不同意退款'
  })
  await csRefundAudit({ orderNo: row.orderNo, approve, reason: value })
  ElMessage.success('审核完成')
  await load()
}
const retry = async (row) => {
  await csRefundRetry({ refundNo: row.refundNo })
  ElMessage.success('重试完成')
  await load()
}
const handleAbn = async (row) => {
  const { value } = await ElMessageBox.prompt('处理备注：', '异常收款处理', {
    inputValue: '核实为订单关闭后到达的支付，发起退款'
  })
  await csHandleAbnormal({ channelTxnId: row.channelTxnId, reason: value })
  ElMessage.success('处理完成')
  await load()
}
const setMode = async (refundMode) => {
  const d = await csChannelMode({ refundMode })
  channel.value = d || {}
  ElMessage.success('渠道退款模式已切换为 ' + (d?.refundMode || refundMode))
}

onMounted(async () => { await load() })
</script>

<style scoped>
.mb8 { margin-bottom: 8px; }
.flex-between { display: flex; justify-content: space-between; align-items: center; }
.muted { color: #6b7280; font-size: 12px; }
</style>
