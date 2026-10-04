<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import md5 from "md5";
import QrcodeVue from "qrcode.vue";
import { VueDraggable } from "vue-draggable-plus";
import {
  CopyDocument,
  Delete,
  Edit,
  Link,
  Plus,
  Rank,
  Tickets,
  Search,
} from "@element-plus/icons-vue";
import { AddSub, DelSub, UpdateSub, getSubs } from "@/api/subcription/subs";
import { getNodes } from "@/api/subcription/node";
import { getTemp } from "@/api/subcription/temp";

interface Sub {
  ID: number;
  Name: string;
  Token?: string;
  Config: string | Config;
  Nodes: Node[];
  NodeOrder?: string;
  SubLogs?: SubLog[];
  CreatedAt?: string;
}

interface Node {
  ID: number;
  Name: string;
  Link: string;
  Source?: string;
  SourceName?: string;
  SourceKey?: string;
  SubID?: string;
}

interface Config {
  clash: string;
  surge: string;
  udp: boolean;
  cert: boolean;
}

interface SubLog {
  IP?: string;
  Count?: number;
  Addr?: string;
  Date?: string;
}

interface TemplateFile {
  file: string;
  text: string;
}

interface ClientOption {
  key: string;
  label: string;
  param?: string;
}

const subscriptions = ref<Sub[]>([]);
const nodes = ref<Node[]>([]);
const templates = ref<TemplateFile[]>([]);
const selectedRows = ref<Sub[]>([]);
const listView = ref("cards");
const keyword = ref("");
const editorStep = ref(0);
const editorSteps = ["基本信息", "选择节点", "客户端输出"];
const selectedNodeIds = ref<number[]>([]);
const logs = ref<SubLog[]>([]);
const subscriptionDialogVisible = ref(false);
const clientDialogVisible = ref(false);
const qrDialogVisible = ref(false);
const logsDialogVisible = ref(false);
const subscriptionOptionsOpen = ref<string[]>([]);
const dialogTitle = ref("添加订阅");
const subName = ref("");
const subToken = ref("");
const oldSubName = ref("");
const clashTemplate = ref("./template/clash.yaml");
const surgeTemplate = ref("./template/surge.conf");
const clashTemplateMode = ref<"local" | "url">("local");
const surgeTemplateMode = ref<"local" | "url">("local");
const enabledOptions = ref<string[]>([]);
const currentPage = ref(1);
const pageSize = ref(10);
const clientUrls = ref<Record<string, string>>({});
const qrTitle = ref("");
const qrUrl = ref("");

const clientOptions: ClientOption[] = [
  { key: "auto", label: "自动识别" },
  { key: "clash", label: "Clash Verge / Mihomo", param: "clash" },
  { key: "surge", label: "Surge", param: "surge" },
  { key: "v2ray", label: "V2Ray", param: "v2ray" },
];

const filteredSubscriptions = computed(() =>
  subscriptions.value.filter((row) =>
    row.Name.toLowerCase().includes(keyword.value.trim().toLowerCase())
  )
);
watch(keyword, () => (currentPage.value = 1));
const pagedSubscriptions = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value;
  return filteredSubscriptions.value.slice(start, start + pageSize.value);
});

function selectCard(row: Sub, checked: boolean) {
  selectedRows.value = checked
    ? [...selectedRows.value.filter((item) => item.ID !== row.ID), row]
    : selectedRows.value.filter((item) => item.ID !== row.ID);
}

function goEditorStep(step: number) {
  if (step > 0 && !subName.value.trim()) {
    ElMessage.warning("请先填写订阅名称");
    return;
  }
  editorStep.value = step;
}

const sourceGroups = computed(() => {
  const groups = new Map<string, { name: string; nodes: Node[] }>();
  for (const item of nodes.value) {
    const key = getNodeSourceKey(item);
    if (!groups.has(key))
      groups.set(key, { name: getNodeSourceName(item), nodes: [] });
    groups.get(key)?.nodes.push(item);
  }
  return Array.from(groups.entries())
    .map(([key, item]) => ({
      key,
      name: item.name,
      nodes: item.nodes.sort((a, b) => a.Name.localeCompare(b.Name)),
    }))
    .sort((a, b) => a.name.localeCompare(b.name));
});
onMounted(async () => {
  await Promise.all([loadSubscriptions(), loadNodes(), loadTemplates()]);
});

async function loadSubscriptions() {
  const { data } = await getSubs();
  subscriptions.value = Array.isArray(data) ? data : [];
  selectedRows.value = selectedRows.value.filter((row) =>
    subscriptions.value.some((item) => item.ID === row.ID)
  );
  currentPage.value = Math.min(
    currentPage.value,
    Math.max(1, Math.ceil(filteredSubscriptions.value.length / pageSize.value))
  );
}

async function loadNodes() {
  const { data } = await getNodes();
  nodes.value = Array.isArray(data) ? data : [];
}

async function loadTemplates() {
  const { data } = await getTemp();
  templates.value = Array.isArray(data) ? data : [];
}

function openAddDialog() {
  editorStep.value = 0;
  subscriptionOptionsOpen.value = [];
  dialogTitle.value = "添加订阅";
  subName.value = "";
  subToken.value = generateSubscriptionToken();
  oldSubName.value = "";
  selectedNodeIds.value = [];
  enabledOptions.value = ["udp"];
  clashTemplate.value = "./template/clash.yaml";
  surgeTemplate.value = "./template/surge.conf";
  clashTemplateMode.value = "local";
  surgeTemplateMode.value = "local";
  subscriptionDialogVisible.value = true;
}

function openEditDialog(row: Sub | any) {
  editorStep.value = 0;
  subscriptionOptionsOpen.value = [];
  const config = parseConfig(row.Config);
  dialogTitle.value = "编辑订阅";
  subName.value = row.Name;
  subToken.value = getSubscriptionToken(row);
  oldSubName.value = row.Name;
  selectedNodeIds.value =
    row.Nodes?.map((item: Node) => item.ID).filter(Boolean) ?? [];
  enabledOptions.value = [];
  if (config.udp) enabledOptions.value.push("udp");
  if (config.cert) enabledOptions.value.push("cert");
  clashTemplate.value = config.clash || "./template/clash.yaml";
  surgeTemplate.value = config.surge || "./template/surge.conf";
  clashTemplateMode.value = clashTemplate.value.startsWith("http")
    ? "url"
    : "local";
  surgeTemplateMode.value = surgeTemplate.value.startsWith("http")
    ? "url"
    : "local";
  subscriptionDialogVisible.value = true;
}

async function persistSubscriptionOrder(row: Sub | any) {
  const nodeNames =
    row.Nodes?.map((item: Node) => item.Name).filter(Boolean) ?? [];
  const nodeIds = row.Nodes?.map((item: Node) => item.ID).filter(Boolean) ?? [];
  if (!row.Name || !nodeNames.length) return;

  try {
    await UpdateSub({
      config:
        typeof row.Config === "string"
          ? row.Config
          : JSON.stringify(row.Config),
      name: row.Name,
      oldname: row.Name,
      token: getSubscriptionToken(row),
      nodes: nodeNames.join(","),
      nodeIds: nodeIds.join(","),
    });
    ElMessage.success("节点顺序已保存");
    await loadSubscriptions();
  } catch (error) {
    console.error(error);
    ElMessage.error("保存节点顺序失败");
    await loadSubscriptions();
  }
}

async function submitSubscription() {
  if (!subName.value.trim()) {
    ElMessage.warning("订阅名称不能为空");
    return;
  }
  if (!selectedNodeIds.value.length) {
    ElMessage.warning("请至少选择一个节点");
    return;
  }
  const token = subToken.value.trim().toLowerCase();
  if (!isValidSubscriptionToken(token)) {
    ElMessage.warning(
      "订阅链接标识只能包含 6-64 位小写字母、数字、下划线和短横线"
    );
    return;
  }

  const config: Config = {
    clash: clashTemplate.value.trim(),
    surge: surgeTemplate.value.trim(),
    udp: enabledOptions.value.includes("udp"),
    cert: enabledOptions.value.includes("cert"),
  };

  try {
    if (dialogTitle.value === "添加订阅") {
      await AddSub({
        config: JSON.stringify(config),
        name: subName.value.trim(),
        token,
        nodes: getSelectedNodeNames().join(","),
        nodeIds: selectedNodeIds.value.join(","),
      });
      ElMessage.success("订阅已添加");
    } else {
      await UpdateSub({
        config: JSON.stringify(config),
        name: subName.value.trim(),
        oldname: oldSubName.value,
        token,
        nodes: getSelectedNodeNames().join(","),
        nodeIds: selectedNodeIds.value.join(","),
      });
      ElMessage.success("订阅已更新");
    }
    subscriptionDialogVisible.value = false;
    await loadSubscriptions();
  } catch (error) {
    console.error(error);
    ElMessage.error("保存失败");
  }
}

async function deleteSubscription(row: Sub | any) {
  try {
    await ElMessageBox.confirm(`确定删除「${row.Name}」吗？`, "删除订阅", {
      confirmButtonText: "删除",
      cancelButtonText: "取消",
      type: "warning",
    });
    await DelSub({ id: row.ID });
    ElMessage.success("订阅已删除");
    await loadSubscriptions();
  } catch (error) {
    if (error !== "cancel") {
      console.error(error);
      ElMessage.error("删除失败");
    }
  }
}

async function deleteSelected() {
  if (!selectedRows.value.length) {
    ElMessage.warning("请选择要删除的订阅");
    return;
  }
  try {
    await ElMessageBox.confirm(
      `确定删除选中的 ${selectedRows.value.length} 个订阅吗？`,
      "批量删除",
      {
        confirmButtonText: "删除",
        cancelButtonText: "取消",
        type: "warning",
      }
    );
    await Promise.all(
      selectedRows.value.map((item) => DelSub({ id: item.ID }))
    );
    ElMessage.success("已删除选中订阅");
    await loadSubscriptions();
  } catch (error) {
    if (error !== "cancel") {
      console.error(error);
      ElMessage.error("批量删除失败");
    }
  }
}

function handleSelectionChange(selection: Sub[]) {
  selectedRows.value = selection;
}

function getNodeCount(row: Sub | any) {
  return row.Nodes?.length ?? 0;
}

function formatNodeCount(row: Sub | any) {
  const count = getNodeCount(row);
  return count ? `${count} 个节点` : "暂无节点";
}

function showLogs(row: Sub | any) {
  logs.value = row.SubLogs ?? [];
  logsDialogVisible.value = true;
}

function showClientLinks(row: Sub | any) {
  const baseUrl = `${location.protocol}//${location.host}/c/?token=${encodeURIComponent(getSubscriptionToken(row))}`;
  clientUrls.value = clientOptions.reduce<Record<string, string>>(
    (result, option) => {
      result[option.key] = option.param
        ? `${baseUrl}&client=${option.param}`
        : baseUrl;
      return result;
    },
    {}
  );
  clientDialogVisible.value = true;
}

function getSubscriptionToken(row: Sub | any) {
  return String(row.Token || row.token || md5(row.Name))
    .trim()
    .toLowerCase();
}

function getSelectedNodeNames() {
  const byID = new Map(nodes.value.map((item) => [item.ID, item.Name]));
  return selectedNodeIds.value
    .map((id) => byID.get(id))
    .filter(Boolean) as string[];
}

function generateSubscriptionToken() {
  const bytes = new Uint8Array(8);
  if (window.crypto?.getRandomValues) {
    window.crypto.getRandomValues(bytes);
  } else {
    bytes.forEach((_, index) => {
      bytes[index] = Math.floor(Math.random() * 256);
    });
  }
  return Array.from(bytes)
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
}

function resetSubscriptionToken() {
  subToken.value = generateSubscriptionToken();
}

function isValidSubscriptionToken(token: string) {
  return /^[a-z0-9_-]{6,64}$/.test(token);
}

function getNodeSourceKey(item: Node) {
  const source = String(item.Source || "").trim();
  return source || "manual";
}

function getNodeSourceName(item: Node) {
  const sourceName = String(item.SourceName || "").trim();
  if (sourceName) return sourceName;
  const source = String(item.Source || "").trim();
  if (!source) return "\u624b\u52a8\u8282\u70b9";
  const remoteMatch = source.match(/^3x-ui-source:(\d+)$/);
  if (remoteMatch) return `VPS \u6765\u6e90 ${remoteMatch[1]}`;
  return source;
}

function sourceSelectedCount(sourceKey: string) {
  const selected = new Set(selectedNodeIds.value);
  const group = sourceGroups.value.find((item) => item.key === sourceKey);
  return group?.nodes.filter((item) => selected.has(item.ID)).length ?? 0;
}

function sourceAllSelected(sourceKey: string) {
  const group = sourceGroups.value.find((item) => item.key === sourceKey);
  return (
    !!group?.nodes.length &&
    sourceSelectedCount(sourceKey) === group.nodes.length
  );
}

function addSourceNodes(sourceKey: string) {
  const group = sourceGroups.value.find((item) => item.key === sourceKey);
  if (!group) return;
  const selected = new Set(selectedNodeIds.value);
  const next = [...selectedNodeIds.value];
  for (const item of group.nodes) {
    if (selected.has(item.ID)) continue;
    selected.add(item.ID);
    next.push(item.ID);
  }
  selectedNodeIds.value = next;
}

function removeSourceNodes(sourceKey: string) {
  const group = sourceGroups.value.find((item) => item.key === sourceKey);
  if (!group) return;
  const removeIds = new Set(group.nodes.map((item) => item.ID));
  selectedNodeIds.value = selectedNodeIds.value.filter(
    (id) => !removeIds.has(id)
  );
}

function getNodeNameById(id: number) {
  return nodes.value.find((item) => item.ID === id)?.Name || `节点 ${id}`;
}

function showQr(title: string, url: string) {
  qrTitle.value = title;
  qrUrl.value = url;
  qrDialogVisible.value = true;
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand("copy");
    document.body.removeChild(textarea);
  }
  ElMessage.success("已复制到剪贴板");
}

function openUrl(url: string) {
  window.open(url, "_blank");
}

function parseConfig(value: string | Config): Config {
  if (typeof value !== "string") return value;
  try {
    return JSON.parse(value) as Config;
  } catch {
    return {
      clash: "./template/clash.yaml",
      surge: "./template/surge.conf",
      udp: true,
      cert: false,
    };
  }
}

function formatDate(row: Sub | any) {
  return row.CreatedAt ? new Date(row.CreatedAt).toLocaleString() : "-";
}
</script>

<template>
  <div class="subs-page">
    <section class="page-header">
      <div>
        <span class="page-kicker">SUBSCRIPTION CENTER / 订阅中心</span>
        <h2>我的订阅</h2>
        <p>为 Clash Verge Rev / Mihomo、Surge 和 V2Ray 生成订阅地址。</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openAddDialog"
        >添加订阅</el-button
      >
    </section>

    <div class="subscription-toolbar">
      <span
        ><strong>{{ subscriptions.length }}</strong> 份订阅 · 已选
        {{ selectedRows.length }} 份</span
      >
      <el-input
        v-model="keyword"
        :prefix-icon="Search"
        clearable
        placeholder="搜索订阅名称"
        class="subscription-search"
      />
      <el-radio-group
        v-model="listView"
        size="small"
        aria-label="列表布局"
        @change="selectedRows = []"
      >
        <el-radio-button label="cards">卡片</el-radio-button
        ><el-radio-button label="table">列表</el-radio-button>
      </el-radio-group>
    </div>
    <div v-if="listView === 'cards'" class="subscription-grid">
      <article
        v-for="row in pagedSubscriptions"
        :key="row.ID"
        class="subscription-card"
        :class="{ selected: selectedRows.some((item) => item.ID === row.ID) }"
      >
        <div class="subscription-card-top">
          <span class="subscription-glyph"
            ><el-icon><Link /></el-icon
          ></span>
          <div class="subscription-card-heading">
            <h3>{{ row.Name }}</h3>
            <p class="subscription-card-description">Clash · Surge · V2Ray</p>
          </div>
          <el-checkbox
            :model-value="selectedRows.some((item) => item.ID === row.ID)"
            :aria-label="`选择订阅 ${row.Name}`"
            @change="selectCard(row, !!$event)"
          />
        </div>
        <div class="subscription-card-stat">
          <strong>{{ getNodeCount(row) }}</strong
          ><span>个节点</span
          ><el-tag size="small" effect="plain">{{
            parseConfig(row.Config).udp ? "UDP 已启用" : "UDP 未启用"
          }}</el-tag>
        </div>
        <div class="subscription-card-meta">
          <span>创建于 {{ formatDate(row) }}</span
          ><el-button link type="primary" :icon="Tickets" @click="showLogs(row)"
            >访问记录</el-button
          >
        </div>
        <div class="subscription-card-actions">
          <el-button type="primary" :icon="Link" @click="showClientLinks(row)"
            >订阅地址</el-button
          ><el-button :icon="Edit" @click="openEditDialog(row)">编辑</el-button
          ><el-button
            :icon="Delete"
            aria-label="删除订阅"
            @click="deleteSubscription(row)"
          />
        </div>
      </article>
      <el-empty
        v-if="!pagedSubscriptions.length"
        :description="keyword ? '没有匹配的订阅' : '创建你的第一份订阅'"
      />
    </div>
    <el-card
      shadow="never"
      class="content-card"
      :class="{ 'cards-footer': listView === 'cards' }"
    >
      <el-table
        v-if="listView === 'table'"
        :data="pagedSubscriptions"
        stripe
        row-key="ID"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="48" />
        <el-table-column type="expand" width="54">
          <template #default="{ row }">
            <div class="expanded-node-panel">
              <div class="expanded-node-header">
                <div>
                  <strong>节点顺序</strong>
                  <span>{{ formatNodeCount(row) }}</span>
                </div>
                <el-tag v-if="row.Nodes?.length" effect="plain" round
                  >可排序</el-tag
                >
              </div>

              <VueDraggable
                v-if="row.Nodes?.length"
                v-model="row.Nodes"
                :animation="160"
                ghost-class="ghost"
                handle=".drag-handle"
                class="expanded-node-grid"
                @end="persistSubscriptionOrder(row)"
              >
                <div
                  v-for="(node, index) in row.Nodes"
                  :key="node.ID || node.Name"
                  class="expanded-node-item"
                >
                  <el-icon class="drag-handle"><Rank /></el-icon>
                  <span class="row-number">{{ index + 1 }}</span>
                  <span class="node-title">{{ node.Name }}</span>
                </div>
              </VueDraggable>
              <el-empty v-else description="暂无节点" :image-size="72" />
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="Name" label="订阅名称" min-width="180">
          <template #default="{ row }">
            <div class="sub-name-cell">
              <el-tag effect="plain">{{ row.Name }}</el-tag>
              <span class="token-line"
                >标识：{{ getSubscriptionToken(row) }}</span
              >
            </div>
          </template>
        </el-table-column>
        <el-table-column label="节点数量" width="110">
          <template #default="{ row }">
            <el-tag
              :type="getNodeCount(row) ? 'primary' : 'info'"
              effect="light"
              round
            >
              {{ formatNodeCount(row) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="客户端入口" min-width="180">
          <template #default="{ row }">
            <el-button
              link
              type="primary"
              :icon="Link"
              @click="showClientLinks(row)"
            >
              订阅地址
            </el-button>
          </template>
        </el-table-column>
        <el-table-column
          label="创建时间"
          min-width="180"
          :formatter="formatDate"
          sortable
        />
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button
              link
              type="primary"
              :icon="Tickets"
              @click="showLogs(row)"
              >记录</el-button
            >
            <el-button
              link
              type="primary"
              :icon="Edit"
              @click="openEditDialog(row)"
              >编辑</el-button
            >
            <el-button
              link
              type="danger"
              :icon="Delete"
              @click="deleteSubscription(row)"
              >删除</el-button
            >
          </template>
        </el-table-column>
      </el-table>

      <div class="table-footer">
        <div class="batch-actions">
          <el-button
            type="danger"
            plain
            :disabled="!selectedRows.length"
            :icon="Delete"
            @click="deleteSelected"
            >删除选中</el-button
          >
        </div>
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 30, 40]"
          :total="filteredSubscriptions.length"
          layout="total, sizes, prev, pager, next, jumper"
        />
      </div>
    </el-card>

    <el-dialog
      v-model="subscriptionDialogVisible"
      :title="dialogTitle"
      width="920px"
      top="5vh"
      class="subscription-editor"
    >
      <nav class="editor-steps" aria-label="订阅编辑步骤">
        <button
          v-for="(step, index) in editorSteps"
          :key="step"
          type="button"
          :class="{
            active: editorStep === index,
            complete: editorStep > index,
          }"
          :aria-current="editorStep === index ? 'step' : undefined"
          @click="goEditorStep(index)"
        >
          <span>{{ index + 1 }}</span>
          <div>
            {{ step
            }}<small>{{
              ["名称与用途", "来源与节点顺序", "输出与高级设置"][index]
            }}</small>
          </div>
        </button>
      </nav>
      <el-form label-position="top">
        <section v-show="editorStep === 0" class="form-section">
          <div class="form-section__heading">
            <span class="form-section__number">1</span>
            <div>
              <h3>基本信息</h3>
              <p>给这份订阅起一个容易辨认的名字。</p>
            </div>
          </div>
          <el-form-item label="订阅名称">
            <el-input v-model="subName" placeholder="例如：全部节点" />
          </el-form-item>
          <div class="editor-intro">
            <h3>接下来</h3>
            <p>
              下一步选择节点与顺序，最后设置客户端输出。保存后，就能复制地址或扫码导入。
            </p>
          </div>
        </section>

        <section v-show="editorStep === 1" class="form-section">
          <div class="form-section__heading">
            <span class="form-section__number">2</span>
            <div>
              <h3>
                选择节点
                <span class="selection-count"
                  >已选 {{ selectedNodeIds.length }} 个</span
                >
              </h3>
              <p>先按来源批量添加，也可以搜索并单独选择节点。</p>
            </div>
          </div>
          <el-form-item v-if="sourceGroups.length" label="按来源添加">
            <div class="source-picker">
              <div
                v-for="source in sourceGroups"
                :key="source.key"
                class="source-picker-card"
                :class="{ 'is-selected': sourceSelectedCount(source.key) > 0 }"
              >
                <div class="source-picker-main">
                  <strong>{{ source.name }}</strong>
                  <span
                    >{{ sourceSelectedCount(source.key) }} /
                    {{ source.nodes.length }} 个节点已选</span
                  >
                </div>
                <div class="source-picker-actions">
                  <el-button
                    size="small"
                    type="primary"
                    plain
                    :disabled="sourceAllSelected(source.key)"
                    @click="addSourceNodes(source.key)"
                  >
                    全选
                  </el-button>
                  <el-button
                    size="small"
                    plain
                    :disabled="sourceSelectedCount(source.key) === 0"
                    @click="removeSourceNodes(source.key)"
                  >
                    取消选择
                  </el-button>
                </div>
              </div>
            </div>
          </el-form-item>

          <el-form-item label="搜索并选择节点">
            <el-select
              v-model="selectedNodeIds"
              multiple
              filterable
              collapse-tags
              collapse-tags-tooltip
              :max-collapse-tags="3"
              class="full-width"
              popper-class="node-picker-dropdown"
              placeholder="输入节点名称搜索，可多选"
            >
              <el-option
                v-for="item in nodes"
                :key="item.ID"
                :label="item.Name"
                :value="item.ID"
              >
                <div class="node-option">
                  <span>{{ item.Name }}</span
                  ><small>{{ getNodeSourceName(item) }}</small>
                </div>
              </el-option>
            </el-select>
            <p v-if="!nodes.length" class="field-help">
              还没有节点，请先到<router-link to="/subcription/nodes"
                >「节点库」</router-link
              >添加或同步节点。
            </p>
          </el-form-item>

          <div v-if="selectedNodeIds.length" class="selected-nodes-heading">
            <div>
              <strong>已选节点与顺序</strong>
              <p>拖动手柄排序，保存后客户端按此顺序显示。</p>
            </div>
            <el-button link type="info" @click="selectedNodeIds = []"
              >清空选择</el-button
            >
          </div>
          <div v-if="selectedNodeIds.length">
            <VueDraggable
              v-model="selectedNodeIds"
              :animation="160"
              ghost-class="ghost"
              handle=".drag-handle"
              class="node-order"
            >
              <div
                v-for="(nodeId, index) in selectedNodeIds"
                :key="nodeId"
                class="draggable-item"
              >
                <el-icon class="drag-handle"><Rank /></el-icon>
                <span class="row-number">{{ index + 1 }}</span>
                <span class="node-title">{{ getNodeNameById(nodeId) }}</span>
                <el-button
                  link
                  type="info"
                  :icon="Delete"
                  aria-label="移除此节点"
                  @click="
                    selectedNodeIds = selectedNodeIds.filter(
                      (id) => id !== nodeId
                    )
                  "
                />
              </div>
            </VueDraggable>
          </div>
        </section>

        <section v-show="editorStep === 2" class="form-section output-section">
          <div class="form-section__heading">
            <span class="form-section__number">3</span>
            <div>
              <h3>客户端输出</h3>
              <p>普通使用保持默认即可，保存后可复制对应客户端的订阅地址。</p>
            </div>
          </div>
          <el-checkbox-group v-model="enabledOptions" class="output-options">
            <div>
              <el-checkbox label="udp">启用 UDP</el-checkbox>
              <p class="field-help">
                在输出配置中允许 UDP，常用于游戏、语音和
                QUIC；节点本身也需要支持。
              </p>
            </div>
            <div>
              <el-checkbox label="cert">跳过 TLS 证书校验</el-checkbox>
              <p class="field-help">
                用于自签名证书等场景。正常证书建议保持关闭；不会修复订阅服务器的连接问题。
              </p>
            </div>
          </el-checkbox-group>
        </section>

        <el-collapse
          v-show="editorStep === 2"
          v-model="subscriptionOptionsOpen"
          class="subscription-advanced"
        >
          <el-collapse-item title="高级设置 · 输出模板与链接标识" name="output">
            <p class="field-help">
              模板控制客户端中的分流规则、代理组等内容，不改变节点池。只使用某个客户端时，配置它对应的模板即可。
            </p>
            <div class="template-grid">
              <el-form-item label="Clash / Mihomo 输出模板">
                <el-radio-group v-model="clashTemplateMode"
                  ><el-radio label="local">本地文件</el-radio
                  ><el-radio label="url">远程 URL</el-radio></el-radio-group
                >
                <el-select
                  v-if="clashTemplateMode === 'local'"
                  v-model="clashTemplate"
                  filterable
                  class="full-width"
                  placeholder="选择模板文件"
                >
                  <el-option
                    v-for="template in templates"
                    :key="template.file"
                    :label="template.file"
                    :value="`./template/${template.file}`"
                  />
                </el-select>
                <el-input
                  v-else
                  v-model="clashTemplate"
                  class="full-width"
                  placeholder="https://example.com/clash.yaml"
                />
              </el-form-item>
              <el-form-item label="Surge 输出模板">
                <el-radio-group v-model="surgeTemplateMode"
                  ><el-radio label="local">本地文件</el-radio
                  ><el-radio label="url">远程 URL</el-radio></el-radio-group
                >
                <el-select
                  v-if="surgeTemplateMode === 'local'"
                  v-model="surgeTemplate"
                  filterable
                  class="full-width"
                  placeholder="选择模板文件"
                >
                  <el-option
                    v-for="template in templates"
                    :key="template.file"
                    :label="template.file"
                    :value="`./template/${template.file}`"
                  />
                </el-select>
                <el-input
                  v-else
                  v-model="surgeTemplate"
                  class="full-width"
                  placeholder="https://example.com/surge.conf"
                />
              </el-form-item>
            </div>
            <el-form-item label="订阅链接标识">
              <div class="token-editor">
                <el-input
                  v-model="subToken"
                  maxlength="64"
                  placeholder="自动生成，通常无需修改"
                  @input="subToken = subToken.trim().toLowerCase()"
                />
                <el-button @click="resetSubscriptionToken"
                  >生成新标识</el-button
                >
              </div>
              <p class="field-help">
                这是订阅地址中的唯一标识，与节点 API Token
                无关。更换后旧订阅地址失效，需要在客户端重新导入。
              </p>
            </el-form-item>
          </el-collapse-item>
        </el-collapse>
      </el-form>
      <template #footer>
        <span class="dialog-selection-summary"
          >已选择 {{ selectedNodeIds.length }} 个节点</span
        >
        <el-button @click="subscriptionDialogVisible = false">取消</el-button>
        <el-button v-if="editorStep > 0" @click="goEditorStep(editorStep - 1)"
          >上一步</el-button
        >
        <el-button
          v-if="editorStep < 2"
          type="primary"
          @click="goEditorStep(editorStep + 1)"
          >下一步</el-button
        >
        <el-button v-else type="primary" @click="submitSubscription"
          >保存订阅</el-button
        >
      </template>
    </el-dialog>

    <el-dialog
      v-model="clientDialogVisible"
      title="客户端订阅地址"
      width="680px"
    >
      <div class="client-list">
        <div
          v-for="option in clientOptions"
          :key="option.key"
          class="client-row"
        >
          <div>
            <strong>{{ option.label }}</strong>
            <p>{{ clientUrls[option.key] }}</p>
          </div>
          <div class="client-actions">
            <el-button
              :icon="CopyDocument"
              @click="copyText(clientUrls[option.key])"
              >复制</el-button
            >
            <el-button @click="showQr(option.label, clientUrls[option.key])"
              >二维码</el-button
            >
            <el-button type="primary" @click="openUrl(clientUrls[option.key])"
              >打开</el-button
            >
          </div>
        </div>
      </div>
    </el-dialog>

    <el-dialog
      v-model="qrDialogVisible"
      :title="qrTitle"
      width="360px"
      class="qr-dialog"
    >
      <div class="qr-box">
        <qrcode-vue :value="qrUrl" :size="220" level="H" />
        <el-input
          v-model="qrUrl"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 4 }"
        />
        <div class="client-actions">
          <el-button :icon="CopyDocument" @click="copyText(qrUrl)"
            >复制</el-button
          >
          <el-button type="primary" @click="openUrl(qrUrl)">打开</el-button>
        </div>
      </div>
    </el-dialog>

    <el-dialog v-model="logsDialogVisible" title="访问记录" width="760px">
      <el-table :data="logs" border>
        <el-table-column prop="IP" label="IP" min-width="150" />
        <el-table-column prop="Count" label="访问次数" width="110" />
        <el-table-column prop="Addr" label="来源" min-width="180" />
        <el-table-column prop="Date" label="最近访问" min-width="180" />
      </el-table>
    </el-dialog>
  </div>
</template>

<style scoped>
.subs-page {
  min-height: 100%;
  padding: 20px;
  color: var(--sx-text);
  background:
    linear-gradient(
      180deg,
      rgba(239, 246, 255, 0.9),
      rgba(248, 250, 252, 0.4) 260px
    ),
    #f6f8fb;
}

.page-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  padding: 24px;
  margin-bottom: 16px;
  border: 1px solid var(--sx-border);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
}

.page-header h2 {
  margin: 0 0 6px;
  color: var(--sx-text);
  font-size: 26px;
  font-weight: 750;
  letter-spacing: 0;
}

.page-header p {
  margin: 0;
  color: var(--sx-muted);
}

.content-card {
  border: 1px solid var(--sx-border);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
}

.content-card :deep(.el-card__body) {
  padding: 18px;
}

.content-card :deep(.el-table) {
  overflow: hidden;
  border: 1px solid var(--sx-border);
  border-radius: 8px;
}

.content-card :deep(.el-table th.el-table__cell) {
  background: var(--sx-page);
  color: var(--sx-muted);
}

.content-card :deep(.el-table__expand-icon) {
  width: 30px;
  height: 30px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #eff6ff;
  color: #2563eb;
}

.content-card :deep(.el-table__expanded-cell) {
  padding: 0 !important;
  background: var(--sx-page);
}

.expanded-node-panel {
  margin: 12px 18px 18px;
  padding: 16px;
  border: 1px solid var(--sx-border);
  border-radius: 8px;
  background: var(--sx-surface);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.8);
}

.expanded-node-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
  margin-bottom: 12px;
  border-bottom: 1px solid var(--sx-border);
}

.expanded-node-header strong {
  display: block;
  margin-bottom: 3px;
  color: var(--sx-text);
  font-size: 15px;
}

.expanded-node-header span {
  color: var(--sx-muted);
  font-size: 13px;
}

.expanded-node-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
  gap: 10px;
}

.expanded-node-item {
  display: grid;
  grid-template-columns: 20px 28px minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  min-height: 44px;
  padding: 9px 12px;
  cursor: grab;
  border: 1px solid var(--sx-border);
  border-radius: 8px;
  background: var(--sx-surface);
  transition:
    border-color 0.16s ease,
    box-shadow 0.16s ease,
    transform 0.16s ease;
}

.table-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 16px;
}

.sub-name-cell {
  display: grid;
  gap: 6px;
  justify-items: start;
}

.token-line {
  max-width: 260px;
  overflow: hidden;
  color: var(--sx-muted);
  font-family:
    ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono",
    monospace;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.full-width {
  width: 100%;
  margin-top: 10px;
}

.token-editor {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  width: 100%;
}

.source-picker {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 10px;
  width: 100%;
}

.source-picker-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
  padding: 12px;
  border: 1px solid var(--sx-border);
  border-radius: 8px;
  background: var(--sx-surface);
}

.source-picker-main {
  min-width: 0;
}

.source-picker-main strong {
  display: block;
  overflow: hidden;
  color: var(--sx-text);
  font-size: 14px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.source-picker-main span {
  display: block;
  margin-top: 3px;
  color: var(--sx-muted);
  font-size: 13px;
}

.source-picker-actions {
  display: flex;
  gap: 8px;
}

.node-order {
  width: 100%;
  max-height: 260px;
  overflow-y: auto;
  padding-right: 4px;
}

.sort-helper {
  width: 100%;
  margin: -2px 0 10px;
  color: var(--sx-muted);
  font-size: 13px;
}

.draggable-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  margin-bottom: 8px;
  cursor: grab;
  background: var(--sx-page);
  border: 1px solid var(--sx-border);
  border-radius: 8px;
  transition:
    border-color 0.16s ease,
    box-shadow 0.16s ease,
    transform 0.16s ease;
}

.draggable-item:hover,
.expanded-node-item:hover {
  border-color: #bfdbfe;
  box-shadow: 0 8px 20px rgba(37, 99, 235, 0.1);
  transform: translateY(-1px);
}

.drag-handle {
  flex: 0 0 auto;
  color: var(--sx-muted);
  cursor: grab;
  font-size: 18px;
}

.row-number {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  font-size: 12px;
  color: var(--sx-muted);
  background: var(--sx-border);
  border-radius: 8px;
}

.row-number.compact {
  width: 22px;
  height: 22px;
}

.node-title {
  min-width: 0;
  overflow: hidden;
  color: var(--sx-text);
  font-weight: 400;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ghost {
  opacity: 0.55;
  background: #dbeafe;
}

.expanded-node-item .drag-handle {
  font-size: 15px;
}

.empty-text {
  color: var(--sx-muted);
}

.client-list {
  display: grid;
  gap: 12px;
}

.client-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px;
  border: 1px solid var(--sx-border);
  border-radius: 8px;
  background: var(--sx-page);
}

.client-row p {
  max-width: 390px;
  margin: 4px 0 0;
  overflow: hidden;
  color: var(--el-text-color-secondary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.client-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.qr-box {
  display: grid;
  gap: 12px;
  justify-items: center;
}

.dialog-selection-summary {
  margin-right: auto;
  color: var(--sx-muted);
  font-size: 13px;
}
.selection-count {
  display: inline-block;
  margin-left: 8px;
  color: var(--el-color-primary);
  font-size: 13px;
  font-weight: 400;
}
.source-picker-card.is-selected {
  border-color: var(--sx-border);
  background: var(--sx-accent-soft);
}
.selected-nodes-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.selected-nodes-heading strong {
  font-size: 14px;
  font-weight: 500;
}
.selected-nodes-heading p {
  margin: 5px 0 0;
  color: var(--sx-muted);
  font-size: 13px;
}
.draggable-item .node-title {
  flex: 1;
  line-height: 1.6;
}
.node-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  font-size: 14px;
}
.node-option span {
  overflow: hidden;
  text-overflow: ellipsis;
}
.node-option small {
  flex-shrink: 0;
  color: var(--sx-muted);
  font-size: 12px;
}
.output-options,
.template-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}
.subscription-advanced {
  border-top: 0;
}
.subscription-advanced .template-grid {
  margin-top: 18px;
}
.output-section {
  border-bottom: 0;
  padding-bottom: 0;
}

@media (max-width: 760px) {
  .source-picker,
  .output-options,
  .template-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .source-picker-actions {
    flex-wrap: wrap;
  }
  .page-header,
  .table-footer,
  .client-row,
  .source-picker-card,
  .token-editor {
    display: block;
  }

  .expanded-node-header {
    display: block;
  }

  .expanded-node-grid {
    grid-template-columns: 1fr;
  }

  .page-header .el-button,
  .batch-actions,
  .client-actions,
  .source-picker-actions,
  .token-editor .el-button {
    margin-top: 12px;
  }
}
</style>
