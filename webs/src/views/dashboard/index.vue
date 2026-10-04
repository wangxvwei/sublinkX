<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import {
  ArrowRight,
  Collection,
  Connection,
  DataLine,
  Link,
  Refresh,
} from "@element-plus/icons-vue";
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
const ungroupedNodeCount = computed(() =>
  Math.max(nodeTotal.value - groupedNodeCount.value, 0)
);
const xhttpNodeCount = computed(
  () => nodes.value.filter((item) => isXhttpNode(item.Link)).length
);
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
    hint: "已创建的订阅链接",
    icon: Collection,
  },
  {
    label: "节点",
    value: nodeTotal.value,
    hint: "节点库中的全部节点",
    icon: Link,
  },
  {
    label: "分组",
    value: groups.value.length,
    hint: "用于整理节点和订阅",
    icon: Connection,
  },
  {
    label: "xhttp 节点",
    value: xhttpNodeCount.value,
    hint: "使用 xhttp 传输的节点",
    icon: DataLine,
  },
]);

onMounted(() => {
  refreshDashboard();
});

async function refreshDashboard() {
  loading.value = true;
  try {
    const [
      subTotalResult,
      nodeTotalResult,
      nodesResult,
      groupsResult,
      subsResult,
    ] = await Promise.allSettled([
      getSubTotal(),
      getNodeTotal(),
      getNodes(),
      GetGroup(),
      getSubs(),
    ]);

    if (subTotalResult.status === "fulfilled")
      subTotal.value = Number(subTotalResult.value.data ?? 0);
    if (nodeTotalResult.status === "fulfilled")
      nodeTotal.value = Number(nodeTotalResult.value.data ?? 0);
    if (nodesResult.status === "fulfilled") {
      nodes.value = Array.isArray(nodesResult.value.data)
        ? nodesResult.value.data
        : [];
      if (!nodeTotal.value) nodeTotal.value = nodes.value.length;
    }
    if (groupsResult.status === "fulfilled") {
      groups.value = Array.isArray(groupsResult.value.data)
        ? groupsResult.value.data
        : [];
    }
    if (subsResult.status === "fulfilled") {
      subs.value = Array.isArray(subsResult.value.data)
        ? subsResult.value.data
        : [];
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
  if (row.NodeOrderIDs)
    return row.NodeOrderIDs.split(",").filter(Boolean).length;
  if (row.NodeOrder) return row.NodeOrder.split(",").filter(Boolean).length;
  return 0;
}

function goNodes() {
  router.push("/subcription/nodes");
}
</script>

<template>
  <div class="dashboard-page" v-loading="loading">
    <header class="overview-header">
      <div>
        <h1>总览</h1>
        <p>{{ greeting }}，这里是你的订阅与节点概况。</p>
      </div>
      <div class="overview-actions">
        <el-button :icon="Refresh" @click="refreshDashboard">刷新</el-button>
        <el-button
          type="primary"
          :icon="Collection"
          @click="router.push('/subcription/subs')"
          >管理订阅</el-button
        >
      </div>
    </header>

    <section class="metric-grid">
      <div v-for="item in metrics" :key="item.label" class="metric-card">
        <div class="metric-label">
          <span>{{ item.label }}</span>
          <el-icon><component :is="item.icon" /></el-icon>
        </div>
        <strong>{{ item.value }}</strong>
        <p>{{ item.hint }}</p>
      </div>
    </section>

    <nav class="quick-access" aria-label="常用入口">
      <router-link to="/resources/sources">
        <el-icon class="quick-icon"><Connection /></el-icon>
        <div>
          <strong>来源接入</strong><span>管理 API 与 SSH 来源，同步节点</span>
        </div>
        <el-icon class="quick-arrow"><ArrowRight /></el-icon>
      </router-link>
      <router-link to="/subcription/nodes">
        <el-icon class="quick-icon"><Link /></el-icon>
        <div>
          <strong>节点库</strong><span>导入节点、调整分组与节点配置</span>
        </div>
        <el-icon class="quick-arrow"><ArrowRight /></el-icon>
      </router-link>
      <router-link to="/subcription/subs">
        <el-icon class="quick-icon"><Collection /></el-icon>
        <div>
          <strong>我的订阅</strong><span>组合节点，生成客户端订阅链接</span>
        </div>
        <el-icon class="quick-arrow"><ArrowRight /></el-icon>
      </router-link>
    </nav>

    <section class="dashboard-grid">
      <div class="overview-panel node-panel">
        <div class="overview-panel-head">
          <div>
            <h2>最近节点</h2>
            <p>节点库中的前 {{ recentNodes.length }} 个节点</p>
          </div>
          <el-button link type="primary" @click="goNodes"
            >查看全部 <el-icon><ArrowRight /></el-icon
          ></el-button>
        </div>
        <div v-if="recentNodes.length" class="overview-table">
          <div class="overview-table-label">
            <span>名称 / 分组</span><span>协议</span>
          </div>
          <div
            v-for="item in recentNodes"
            :key="item.ID"
            class="overview-node-row"
          >
            <div class="node-identity">
              <strong>{{ item.Name }}</strong>
              <span>{{ formatGroups(item) }}</span>
            </div>
            <el-tag effect="plain">{{
              isXhttpNode(item.Link) ? "VLESS xhttp" : getProtocol(item.Link)
            }}</el-tag>
          </div>
        </div>
        <el-empty
          v-else
          description="还没有节点，先接入来源或导入节点"
          :image-size="80"
        />
      </div>

      <div class="overview-panel protocol-panel">
        <div class="overview-panel-head">
          <div>
            <h2>协议分布</h2>
            <p>按节点链接的协议类型统计</p>
          </div>
        </div>
        <div v-if="protocolStats.length" class="protocol-list">
          <div
            v-for="item in protocolStats"
            :key="item.name"
            class="protocol-row"
          >
            <span>{{ item.name }}</span>
            <div class="protocol-bar">
              <i
                :style="{
                  width: `${(item.count / Math.max(nodeTotal, 1)) * 100}%`,
                }"
              />
            </div>
            <strong>{{ item.count }}</strong>
          </div>
        </div>
        <el-empty v-else description="暂无节点数据" :image-size="80" />
      </div>

      <div class="overview-panel subscription-panel">
        <div class="overview-panel-head">
          <div>
            <h2>我的订阅</h2>
            <p>选择订阅后，可编辑节点和复制链接</p>
          </div>
          <el-button
            link
            type="primary"
            @click="router.push('/subcription/subs')"
            >管理订阅 <el-icon><ArrowRight /></el-icon
          ></el-button>
        </div>
        <div v-if="recentSubs.length" class="overview-table">
          <div class="overview-table-label">
            <span>订阅名称</span><span>节点数量</span>
          </div>
          <router-link
            v-for="item in recentSubs"
            :key="item.ID"
            to="/subcription/subs"
            class="overview-subscription-row"
          >
            <strong>{{ item.Name }}</strong>
            <span
              >{{ getSubscriptionNodeCount(item) }} 个节点
              <el-icon><ArrowRight /></el-icon
            ></span>
          </router-link>
        </div>
        <el-empty
          v-else
          description="还没有订阅，前往「我的订阅」创建"
          :image-size="80"
        />
      </div>

      <div class="overview-panel group-panel">
        <div class="overview-panel-head">
          <div>
            <h2>节点分组</h2>
            <p>分组可用于批量添加订阅节点</p>
          </div>
        </div>
        <div class="status-list">
          <div class="status-row">
            <span><i class="status-dot grouped"></i>已分组</span>
            <strong>{{ groupedNodeCount }} <small>个</small></strong>
          </div>
          <el-progress
            :percentage="
              nodeTotal ? Math.round((groupedNodeCount / nodeTotal) * 100) : 0
            "
            :show-text="false"
          />
          <div class="status-row">
            <span><i class="status-dot ungrouped"></i>未分组</span>
            <strong>{{ ungroupedNodeCount }} <small>个</small></strong>
          </div>
          <el-progress
            color="var(--sx-muted)"
            :percentage="
              nodeTotal ? Math.round((ungroupedNodeCount / nodeTotal) * 100) : 0
            "
            :show-text="false"
          />
        </div>
        <p class="group-note">此处仅统计分组情况，不代表节点连通性。</p>
      </div>
    </section>
  </div>
</template>

<style lang="scss" scoped>
.dashboard-page {
  min-height: 100%;
  padding: 28px;
  color: var(--sx-text);
  background: var(--sx-page);
}

.overview-panel,
.metric-card {
  border: 1px solid var(--sx-border);
  border-radius: 10px;
  background: var(--sx-surface);
}

.overview-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 24px;
}

.overview-header h1 {
  margin: 0;
  font-size: 25px;
  font-weight: 650;
  letter-spacing: -0.5px;
}

.overview-header p,
.overview-panel-head p,
.metric-card p,
.node-identity span,
.overview-subscription-row span {
  margin: 6px 0 0;
  color: var(--sx-muted);
  font-size: 13px;
  line-height: 1.6;
}

.overview-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;

  .el-button + .el-button {
    margin-left: 0;
  }
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 18px;
}

.metric-card {
  padding: 18px 20px;
}

.metric-label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  color: var(--sx-muted);
  font-size: 14px;
}

.metric-label .el-icon {
  color: var(--el-color-primary);
  font-size: 19px;
}

.metric-card strong {
  display: block;
  margin-top: 14px;
  color: var(--sx-text);
  font-size: 32px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
}

.quick-access {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin-bottom: 24px;
  border: 1px solid var(--sx-border);
  border-radius: 10px;
  background: var(--sx-surface);
  overflow: hidden;

  a {
    display: flex;
    align-items: center;
    min-width: 0;
    gap: 12px;
    padding: 18px 20px;
    transition: background-color 0.15s;
  }
  a + a {
    border-left: 1px solid var(--sx-border);
  }
  a:hover {
    background: var(--sx-accent-soft);
  }
  a:focus-visible {
    outline: 2px solid var(--el-color-primary);
    outline-offset: -3px;
  }
  strong {
    display: block;
    color: var(--sx-text);
    font-size: 14px;
    font-weight: 600;
  }
  span {
    display: block;
    margin-top: 4px;
    color: var(--sx-muted);
    font-size: 12px;
    line-height: 1.5;
  }
  .quick-icon {
    font-size: 21px;
    color: var(--el-color-primary);
    flex-shrink: 0;
  }
  .quick-arrow {
    margin-left: auto;
    color: var(--sx-muted);
    flex-shrink: 0;
  }
}

.dashboard-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.65fr) minmax(280px, 1fr);
  grid-template-areas: "nodes protocol" "subscriptions groups";
  align-items: start;
  gap: 18px;
}

.node-panel {
  grid-area: nodes;
}
.protocol-panel {
  grid-area: protocol;
}
.subscription-panel {
  grid-area: subscriptions;
}
.group-panel {
  grid-area: groups;
}

.overview-panel {
  min-width: 0;
}

.overview-panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 20px 20px 16px;

  .el-button {
    flex-shrink: 0;
  }
  .el-button .el-icon {
    margin-left: 5px;
  }
}

.overview-panel-head h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 650;
}

.status-list,
.protocol-list {
  display: grid;
  gap: 16px;
  padding: 4px 20px 24px;
}

.status-row,
.protocol-row,
.overview-table-label,
.overview-node-row,
.overview-subscription-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.status-row strong,
.protocol-row strong {
  color: var(--sx-text);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.protocol-row span {
  width: 92px;
  font-size: 12px;
  color: var(--sx-text);
  overflow-wrap: anywhere;
}

.protocol-bar {
  flex: 1;
  height: 6px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--sx-border);
}

.protocol-bar i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--el-color-primary);
}

.overview-table-label {
  padding: 9px 20px;
  color: var(--sx-muted);
  background: var(--sx-page);
  border-top: 1px solid var(--sx-border);
  border-bottom: 1px solid var(--sx-border);
  font-size: 12px;
}

.overview-node-row,
.overview-subscription-row {
  padding: 13px 20px;
  min-height: 58px;
}

.overview-node-row + .overview-node-row,
.overview-subscription-row + .overview-subscription-row {
  border-top: 1px solid var(--sx-border);
}
.overview-subscription-row:hover {
  background: var(--sx-accent-soft);
}
.overview-subscription-row:last-child {
  border-radius: 0 0 10px 10px;
}
.overview-node-row .el-tag {
  flex-shrink: 0;
}
.node-identity {
  min-width: 0;
}
.node-identity span {
  display: block;
  margin-top: 3px;
  overflow-wrap: anywhere;
}
.overview-node-row strong,
.overview-subscription-row strong {
  display: block;
  color: var(--sx-text);
  font-size: 14px;
  font-weight: 500;
  overflow-wrap: anywhere;
}
.overview-subscription-row span {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 0;
  flex-shrink: 0;
}
.status-row {
  font-size: 13px;
}
.status-row span {
  display: flex;
  align-items: center;
  gap: 8px;
}
.status-row small {
  color: var(--sx-muted);
  font-weight: 400;
}
.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--sx-muted);
}
.status-dot.grouped {
  background: var(--el-color-primary);
}
.group-note {
  margin: 0;
  padding: 0 20px 20px;
  color: var(--sx-muted);
  font-size: 12px;
  line-height: 1.6;
}

@media (max-width: 1100px) {
  .dashboard-page {
    padding: 22px;
  }
  .metric-grid {
    gap: 12px;
  }
  .metric-card {
    padding: 16px;
  }
  .quick-access a {
    padding: 16px;
  }
  .quick-access .quick-icon {
    display: none;
  }
}

@media (max-width: 760px) {
  .dashboard-page {
    padding: 16px;
  }
  .overview-header {
    align-items: flex-start;
    flex-wrap: wrap;
    margin-bottom: 18px;
  }
  .overview-header h1 {
    font-size: 22px;
  }
  .overview-header p {
    font-size: 12px;
  }
  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .metric-card strong {
    font-size: 28px;
  }
  .metric-card p {
    font-size: 12px;
  }
  .quick-access {
    grid-template-columns: 1fr;
    margin-bottom: 18px;
  }
  .quick-access a {
    padding: 14px 16px;
  }
  .quick-access a + a {
    border-left: 0;
    border-top: 1px solid var(--sx-border);
  }
  .quick-access .quick-icon {
    display: inline-flex;
  }
  .dashboard-grid {
    grid-template-columns: minmax(0, 1fr);
    grid-template-areas: "nodes" "subscriptions" "protocol" "groups";
  }
}
</style>
