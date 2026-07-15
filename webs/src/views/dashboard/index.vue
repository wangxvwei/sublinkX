<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { Collection, Connection, DataLine, Link, Refresh } from "@element-plus/icons-vue";
import { getNodeTotal, getSubTotal } from "@/api/total";
import { getNodes, GetGroup } from "@/api/subcription/node";
import { getSubs } from "@/api/subcription/subs";
import { useUserStore } from "@/store/modules/user";

defineOptions({
  name: "Dashboard",
  inheritAttrs: false,
});

interface GroupNode {
  ID?: number;
  Name: string;
}

interface NodeItem {
  ID: number;
  Name: string;
  Link: string;
  CreatedAt?: string;
  CreateDate?: string;
  GroupNodes?: GroupNode[];
}

interface SubItem {
  ID: number;
  Name: string;
  Nodes?: NodeItem[];
  NodeOrder?: string;
  NodeOrderIDs?: string;
  CreatedAt?: string;
}

const userStore = useUserStore();
const router = useRouter();
const loading = ref(false);
const subTotal = ref(0);
const nodeTotal = ref(0);
const nodes = ref<NodeItem[]>([]);
const subs = ref<SubItem[]>([]);
const groups = ref<string[]>([]);

const hour = new Date().getHours();
const greeting = computed(() => {
  const name = userStore.user.nickname || userStore.user.username || "管理员";
  if (hour < 6) return `夜深了，${name}`;
  if (hour < 12) return `早上好，${name}`;
  if (hour < 18) return `下午好，${name}`;
  return `晚上好，${name}`;
});

const groupedNodeCount = computed(
  () => nodes.value.filter((item) => (item.GroupNodes?.length ?? 0) > 0).length
);
const ungroupedNodeCount = computed(() => Math.max(nodeTotal.value - groupedNodeCount.value, 0));
const xhttpNodeCount = computed(() => nodes.value.filter((item) => isXhttpNode(item.Link)).length);
const protocolStats = computed(() => {
  const map = new Map<string, number>();
  nodes.value.forEach((item) => {
    const key = getProtocol(item.Link);
    map.set(key, (map.get(key) ?? 0) + 1);
  });
  return Array.from(map.entries())
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 6);
});
const recentNodes = computed(() => nodes.value.slice(0, 6));
const recentSubs = computed(() => subs.value.slice(0, 5));

const metrics = computed(() => [
  {
    label: "订阅",
    value: subTotal.value,
    hint: "可复制给客户端的订阅入口",
    icon: Collection,
    tone: "blue",
  },
  {
    label: "节点",
    value: nodeTotal.value,
    hint: "当前维护的全部节点",
    icon: Link,
    tone: "green",
  },
  {
    label: "分组",
    value: groups.value.length,
    hint: "用于整理节点和订阅",
    icon: Connection,
    tone: "amber",
  },
  {
    label: "xhttp",
    value: xhttpNodeCount.value,
    hint: "Clash Verge / Mihomo 可用",
    icon: DataLine,
    tone: "rose",
  },
]);

onMounted(() => {
  refreshDashboard();
});

async function refreshDashboard() {
  loading.value = true;
  try {
    const [subTotalResult, nodeTotalResult, nodesResult, groupsResult, subsResult] =
      await Promise.allSettled([getSubTotal(), getNodeTotal(), getNodes(), GetGroup(), getSubs()]);

    if (subTotalResult.status === "fulfilled") subTotal.value = Number(subTotalResult.value.data ?? 0);
    if (nodeTotalResult.status === "fulfilled") nodeTotal.value = Number(nodeTotalResult.value.data ?? 0);
    if (nodesResult.status === "fulfilled") {
      nodes.value = Array.isArray(nodesResult.value.data) ? nodesResult.value.data : [];
      if (!nodeTotal.value) nodeTotal.value = nodes.value.length;
    }
    if (groupsResult.status === "fulfilled") {
      groups.value = Array.isArray(groupsResult.value.data) ? groupsResult.value.data : [];
    }
    if (subsResult.status === "fulfilled") {
      subs.value = Array.isArray(subsResult.value.data) ? subsResult.value.data : [];
      if (!subTotal.value) subTotal.value = subs.value.length;
    }
  } finally {
    loading.value = false;
  }
}

function getProtocol(link: string) {
  const scheme = link.split("://")[0]?.trim().toUpperCase();
  return scheme || "UNKNOWN";
}

function decodeLinkBody(link: string) {
  const body = link.split("://")[1] ?? "";
  try {
    return decodeURIComponent(atob(body));
  } catch {
    return decodeURIComponent(link);
  }
}

function isXhttpNode(link: string) {
  return decodeLinkBody(link).toLowerCase().includes("type=xhttp");
}

function formatGroups(row: NodeItem) {
  const names = row.GroupNodes?.map((item) => item.Name).filter(Boolean) ?? [];
  return names.length ? names.join(" / ") : "未分组";
}

function getSubscriptionNodeCount(row: SubItem) {
  if (row.Nodes?.length) return row.Nodes.length;
  if (row.NodeOrderIDs) return row.NodeOrderIDs.split(",").filter(Boolean).length;
  if (row.NodeOrder) return row.NodeOrder.split(",").filter(Boolean).length;
  return 0;
}

function goNodes() {
  router.push("/subcription/nodes");
}
</script>

<template>
  <div class="dashboard-page" v-loading="loading">
    <section class="hero-panel">
      <div class="hero-copy">
        <span class="eyebrow">sublinkX 控制台</span>
        <h1>{{ greeting }}</h1>
        <p>这里集中展示订阅、节点、分组和 xhttp 支持情况。你可以从首页快速判断当前配置是否适合 Clash Verge Rev / Mihomo。</p>
      </div>
      <div class="hero-actions">
        <el-button :icon="Refresh" @click="refreshDashboard">刷新数据</el-button>
        <el-button type="primary" :icon="Link" @click="goNodes">管理节点</el-button>
      </div>
    </section>

    <section class="metric-grid">
      <div v-for="item in metrics" :key="item.label" class="metric-card" :class="`tone-${item.tone}`">
        <div class="metric-icon">
          <el-icon><component :is="item.icon" /></el-icon>
        </div>
        <div>
          <span>{{ item.label }}</span>
          <strong>{{ item.value }}</strong>
          <p>{{ item.hint }}</p>
        </div>
      </div>
    </section>

    <section class="dashboard-grid">
      <div class="panel">
        <div class="panel-head">
          <div>
            <h2>节点健康概览</h2>
            <p>按维护状态快速看待整理的节点。</p>
          </div>
        </div>
        <div class="status-list">
          <div class="status-row">
            <span>已分组节点</span>
            <strong>{{ groupedNodeCount }}</strong>
          </div>
          <el-progress :percentage="nodeTotal ? Math.round((groupedNodeCount / nodeTotal) * 100) : 0" :show-text="false" />
          <div class="status-row">
            <span>未分组节点</span>
            <strong>{{ ungroupedNodeCount }}</strong>
          </div>
          <el-progress status="warning" :percentage="nodeTotal ? Math.round((ungroupedNodeCount / nodeTotal) * 100) : 0" :show-text="false" />
        </div>
      </div>

      <div class="panel">
        <div class="panel-head">
          <div>
            <h2>协议分布</h2>
            <p>帮助你发现订阅里主要的节点类型。</p>
          </div>
        </div>
        <div v-if="protocolStats.length" class="protocol-list">
          <div v-for="item in protocolStats" :key="item.name" class="protocol-row">
            <span>{{ item.name }}</span>
            <div class="protocol-bar">
              <i :style="{ width: `${Math.max((item.count / Math.max(nodeTotal, 1)) * 100, 8)}%` }" />
            </div>
            <strong>{{ item.count }}</strong>
          </div>
        </div>
        <el-empty v-else description="暂无节点数据" :image-size="80" />
      </div>

      <div class="panel wide">
        <div class="panel-head">
          <div>
            <h2>最近节点</h2>
            <p>节点名称、分组和协议一眼看清。</p>
          </div>
          <el-button link type="primary" @click="goNodes">查看全部</el-button>
        </div>
        <div v-if="recentNodes.length" class="compact-table">
          <div v-for="item in recentNodes" :key="item.ID" class="compact-row">
            <div>
              <strong>{{ item.Name }}</strong>
              <span>{{ formatGroups(item) }}</span>
            </div>
            <el-tag effect="plain" round>{{ isXhttpNode(item.Link) ? "VLESS xhttp" : getProtocol(item.Link) }}</el-tag>
          </div>
        </div>
        <el-empty v-else description="还没有节点" :image-size="80" />
      </div>

      <div class="panel">
        <div class="panel-head">
          <div>
            <h2>订阅列表</h2>
            <p>当前可分发的订阅。</p>
          </div>
        </div>
        <div v-if="recentSubs.length" class="subscription-list">
          <div v-for="item in recentSubs" :key="item.ID" class="subscription-row">
            <strong>{{ item.Name }}</strong>
            <span>{{ getSubscriptionNodeCount(item) }} 个节点</span>
          </div>
        </div>
        <el-empty v-else description="还没有订阅" :image-size="80" />
      </div>
    </section>
  </div>
</template>

<style lang="scss" scoped>
.dashboard-page {
  min-height: 100%;
  padding: 20px;
  color: #1f2937;
  background:
    linear-gradient(180deg, rgba(239, 246, 255, 0.9), rgba(248, 250, 252, 0.4) 260px),
    #f6f8fb;
}

.hero-panel,
.panel,
.metric-card {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
}

.hero-panel {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  padding: 24px;
}

.eyebrow {
  display: block;
  margin-bottom: 8px;
  color: #2563eb;
  font-size: 13px;
  font-weight: 700;
}

.hero-copy h1 {
  margin: 0;
  font-size: 26px;
  font-weight: 750;
}

.hero-copy p,
.panel-head p,
.metric-card p,
.compact-row span,
.subscription-row span {
  margin: 6px 0 0;
  color: #64748b;
}

.hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin: 16px 0;
}

.metric-card {
  display: flex;
  gap: 14px;
  padding: 18px;
}

.metric-card span {
  color: #64748b;
  font-size: 13px;
}

.metric-card strong {
  display: block;
  margin-top: 4px;
  color: #111827;
  font-size: 30px;
  line-height: 1;
}

.metric-icon {
  display: grid;
  width: 42px;
  height: 42px;
  place-items: center;
  border-radius: 8px;
  font-size: 22px;
}

.tone-blue .metric-icon {
  color: #1d4ed8;
  background: #dbeafe;
}

.tone-green .metric-icon {
  color: #047857;
  background: #d1fae5;
}

.tone-amber .metric-icon {
  color: #b45309;
  background: #fef3c7;
}

.tone-rose .metric-icon {
  color: #be123c;
  background: #ffe4e6;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: 1.1fr 1fr 1fr;
  gap: 14px;
}

.panel {
  min-width: 0;
  padding: 18px;
}

.panel.wide {
  grid-column: span 2;
}

.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.panel-head h2 {
  margin: 0;
  font-size: 17px;
  font-weight: 720;
}

.status-list,
.protocol-list,
.compact-table,
.subscription-list {
  display: grid;
  gap: 12px;
}

.status-row,
.protocol-row,
.compact-row,
.subscription-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.status-row strong,
.protocol-row strong {
  color: #111827;
}

.protocol-row span {
  width: 78px;
  color: #334155;
  font-weight: 650;
}

.protocol-bar {
  flex: 1;
  height: 8px;
  overflow: hidden;
  border-radius: 999px;
  background: #e5e7eb;
}

.protocol-bar i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #2563eb, #059669);
}

.compact-row,
.subscription-row {
  padding: 12px 0;
  border-bottom: 1px solid #eef2f7;
}

.compact-row:last-child,
.subscription-row:last-child {
  border-bottom: 0;
}

.compact-row strong,
.subscription-row strong {
  display: block;
  color: #111827;
}

@media (max-width: 1180px) {
  .metric-grid,
  .dashboard-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .panel.wide {
    grid-column: span 2;
  }
}

@media (max-width: 760px) {
  .dashboard-page {
    padding: 12px;
  }

  .hero-panel,
  .panel-head {
    display: block;
  }

  .hero-actions {
    margin-top: 16px;
  }

  .metric-grid,
  .dashboard-grid {
    grid-template-columns: 1fr;
  }

  .panel.wide {
    grid-column: span 1;
  }
}
</style>
