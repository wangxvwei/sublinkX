<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import {
  Check,
  Close,
  Connection,
  CopyDocument,
  Delete,
  EditPen,
  Plus,
  Refresh,
  Search,
  Setting,
} from "@element-plus/icons-vue";
import {
  AddNodes,
  DelNode,
  DelXUISource,
  GetGroup,
  GetXUISources,
  SaveXUISource,
  SetGroup,
  SyncAllXUISources,
  SyncXUINodes,
  SyncXUISource,
  UpdateNode,
  getNodes,
} from "@/api/subcription/node";

interface GroupNode {
  ID: number;
  Name: string;
}

interface NodeItem {
  ID: number;
  Name: string;
  Link: string;
  LinkOverride?: string;
  Source?: string;
  SourceKey?: string;
  SubID?: string;
  CreatedAt?: string;
  CreateDate?: string;
  GroupNodes?: GroupNode[];
}

interface NodeForm {
  ID?: number;
  Name: string;
  Link: string;
  GroupName: string[];
  NewGroupName: string;
}

type SourceAuthType = "password" | "apiToken";

interface XUISource {
  id?: number;
  name: string;
  host: string;
  sshPort: number;
  username: string;
  authType: SourceAuthType;
  password?: string;
  panelBaseUrl?: string;
  apiToken?: string;
  xuiDbPath: string;
  subBaseUrl: string;
  subPath: string;
  groupName: string;
  namePrefix: string;
  rewriteRules: string;
  deleteMissing: boolean;
  enabled: boolean;
  hasPassword?: boolean;
  hasApiToken?: boolean;
  lastSyncStatus?: string;
  lastSyncMessage?: string;
}

interface XUIRewriteRuleRow {
  nameContains: string;
  protocol: string;
  transport: string;
  address: string;
  port: string;
  security: string;
  sni: string;
  host: string;
  fingerprint: string;
  alpn: string[];
  path: string;
  flow: string;
}

interface SyncResult {
  created?: number;
  updated?: number;
  unchanged?: number;
  skipped?: number;
  deleted?: number;
}

const ALL_GROUP_NAME = "全部";

const activePanel = ref<"nodes" | "sources">("nodes");
const activeGroup = ref(ALL_GROUP_NAME);
const nodeKeyword = ref("");
const sourceKeyword = ref("");
const nodeDialogVisible = ref(false);
const nodeDialogMode = ref<"add" | "edit">("add");
const nodeSaving = ref(false);
const sourceSaving = ref(false);
const nodeLoading = ref(false);
const sourceLoading = ref(false);
const syncingAllSources = ref(false);
const syncingLocalXUI = ref(false);
const selectedNodes = ref<NodeItem[]>([]);
const tableRef = ref<any>(null);
const sourceAdvanced = ref(["sync", "rules"]);

const nodes = ref<NodeItem[]>([]);
const groupNames = ref<string[]>([]);
const sources = ref<XUISource[]>([]);
const editingSourceId = ref<number | undefined>();
const rewriteRuleRows = ref<XUIRewriteRuleRow[]>([]);
const lastSyncResult = ref<SyncResult | null>(null);
let syncResultTimer: number | undefined;

const nodeForm = ref<NodeForm>({
  Name: "",
  Link: "",
  GroupName: [],
  NewGroupName: "",
});

const sourceForm = ref<XUISource>(createEmptySource());

const visibleNodes = computed(() => {
  const keyword = nodeKeyword.value.trim().toLowerCase();
  return nodes.value.filter((node) => {
    const groupMatched =
      activeGroup.value === ALL_GROUP_NAME ||
      node.GroupNodes?.some((group) => group.Name === activeGroup.value);
    if (!groupMatched) return false;
    if (!keyword) return true;
    return [node.Name, node.Link, node.Source, node.SubID]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(keyword));
  });
});

const sourceRows = computed(() => {
  const keyword = sourceKeyword.value.trim().toLowerCase();
  if (!keyword) return sources.value;
  return sources.value.filter((source) =>
    [
      source.name,
      source.host,
      source.panelBaseUrl,
      source.groupName,
      source.lastSyncMessage,
    ]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(keyword))
  );
});

const nodeStats = computed(() => {
  const remoteCount = nodes.value.filter((node) => node.Source).length;
  return {
    total: nodes.value.length,
    groups: groupNames.value.length,
    selected: selectedNodes.value.length,
    remote: remoteCount,
  };
});

const sourceStats = computed(() => {
  const enabled = sources.value.filter((source) => source.enabled).length;
  const failed = sources.value.filter((source) => source.lastSyncStatus === "failed").length;
  return {
    total: sources.value.length,
    enabled,
    failed,
  };
});

const nodeDialogTitle = computed(() =>
  nodeDialogMode.value === "add" ? "添加节点" : "编辑节点"
);

onMounted(async () => {
  await loadAll();
});

onUnmounted(() => {
  clearSyncResultTimer();
});

watch(
  () => sourceForm.value.authType,
  (authType) => {
    if (authType === "password") {
      sourceForm.value.panelBaseUrl = "";
      sourceForm.value.apiToken = "";
    } else {
      sourceForm.value.host = "";
      sourceForm.value.username = "";
      sourceForm.value.password = "";
    }
  }
);

async function loadAll() {
  await Promise.all([loadNodes(), loadGroups(), loadSources()]);
}

async function loadNodes() {
  nodeLoading.value = true;
  try {
    const { data } = await getNodes();
    nodes.value = Array.isArray(data) ? data : [];
  } finally {
    nodeLoading.value = false;
  }
}

async function loadGroups() {
  const { data } = await GetGroup();
  groupNames.value = Array.isArray(data) ? data : [];
  if (activeGroup.value !== ALL_GROUP_NAME && !groupNames.value.includes(activeGroup.value)) {
    activeGroup.value = ALL_GROUP_NAME;
  }
}

async function loadSources() {
  sourceLoading.value = true;
  try {
    const { data } = await GetXUISources();
    sources.value = Array.isArray(data) ? data : [];
  } finally {
    sourceLoading.value = false;
  }
}

function createEmptySource(): XUISource {
  return {
    name: "",
    host: "",
    sshPort: 22,
    username: "root",
    authType: "password",
    password: "",
    panelBaseUrl: "",
    apiToken: "",
    xuiDbPath: "/etc/x-ui/x-ui.db",
    subBaseUrl: "",
    subPath: "",
    groupName: "",
    namePrefix: "",
    rewriteRules: "",
    deleteMissing: false,
    enabled: true,
  };
}

function createRewriteRule(): XUIRewriteRuleRow {
  return {
    nameContains: "",
    protocol: "",
    transport: "xhttp",
    address: "",
    port: "",
    security: "tls",
    sni: "",
    host: "",
    fingerprint: "chrome",
    alpn: ["h2", "http/1.1"],
    path: "",
    flow: "",
  };
}

function beginCreateNode() {
  nodeDialogMode.value = "add";
  nodeForm.value = {
    Name: "",
    Link: "",
    GroupName: activeGroup.value === ALL_GROUP_NAME ? [] : [activeGroup.value],
    NewGroupName: "",
  };
  nodeDialogVisible.value = true;
}

function beginEditNode(row: any) {
  nodeDialogMode.value = "edit";
  nodeForm.value = {
    ID: row.ID,
    Name: row.Name,
    Link: row.LinkOverride || row.Link,
    GroupName: row.GroupNodes?.map((group: GroupNode) => group.Name) || [],
    NewGroupName: "",
  };
  nodeDialogVisible.value = true;
}

function getSelectedGroupPayload(form: NodeForm) {
  const groups = [...form.GroupName];
  const newGroup = form.NewGroupName.trim();
  if (newGroup) groups.push(newGroup);
  return Array.from(new Set(groups)).join(",");
}

async function submitNodeForm() {
  const form = nodeForm.value;
  const group = getSelectedGroupPayload(form);
  const isAdd = nodeDialogMode.value === "add";
  const links = form.Link.split(/[\n,]/).map((item) => item.trim()).filter(Boolean);

  if (!form.Link.trim()) {
    ElMessage.warning("请输入节点链接");
    return;
  }
  if (!isAdd && !form.Name.trim()) {
    ElMessage.warning("请输入节点名称");
    return;
  }

  nodeSaving.value = true;
  try {
    if (isAdd) {
      for (const link of links) {
        await AddNodes({ link, group });
      }
      ElMessage.success(`已添加 ${links.length} 个节点`);
    } else {
      await UpdateNode({
        id: form.ID,
        name: form.Name.trim(),
        link: form.Link.trim(),
        group,
      });
      ElMessage.success("节点已更新");
    }
    nodeDialogVisible.value = false;
    await refreshNodeData();
  } catch (error) {
    console.error(error);
    ElMessage.error(isAdd ? "添加节点失败" : "更新节点失败");
  } finally {
    nodeSaving.value = false;
  }
}

async function refreshNodeData() {
  await Promise.all([loadNodes(), loadGroups()]);
}

function handleSelectionChange(rows: NodeItem[]) {
  selectedNodes.value = rows;
}

function selectAllRows() {
  nextTick(() => {
    visibleNodes.value.forEach((row) => tableRef.value?.toggleRowSelection(row, true));
  });
}

function clearSelection() {
  tableRef.value?.clearSelection();
}

async function copyText(text: string, successText = "已复制到剪贴板") {
  try {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(text);
    } else {
      const textarea = document.createElement("textarea");
      textarea.value = text;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      document.body.removeChild(textarea);
    }
    ElMessage.success(successText);
  } catch (error) {
    console.error(error);
    ElMessage.error("复制失败");
  }
}

async function copyNode(row: any) {
  await copyText(row.LinkOverride || row.Link, "节点链接已复制");
}

async function copySelectedNodes() {
  if (!selectedNodes.value.length) {
    ElMessage.warning("请选择要复制的节点");
    return;
  }
  await copyText(selectedNodes.value.map((node) => node.LinkOverride || node.Link).join("\n"));
}

async function deleteNode(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除「${row.Name}」吗？`, "删除节点", {
      confirmButtonText: "删除",
      cancelButtonText: "取消",
      type: "warning",
    });
    await DelNode({ id: row.ID });
    ElMessage.success("节点已删除");
    await refreshNodeData();
  } catch (error) {
    if (error !== "cancel") {
      console.error(error);
      ElMessage.error("删除节点失败");
    }
  }
}

async function deleteSelectedNodes() {
  if (!selectedNodes.value.length) {
    ElMessage.warning("请选择要删除的节点");
    return;
  }
  try {
    await ElMessageBox.confirm(`确定删除选中的 ${selectedNodes.value.length} 个节点吗？`, "批量删除", {
      confirmButtonText: "删除",
      cancelButtonText: "取消",
      type: "warning",
    });
    for (const node of selectedNodes.value) {
      await DelNode({ id: node.ID });
    }
    ElMessage.success("已删除选中节点");
    clearSelection();
    await refreshNodeData();
  } catch (error) {
    if (error !== "cancel") {
      console.error(error);
      ElMessage.error("批量删除失败");
    }
  }
}

async function bindGroups(row: NodeItem) {
  const group = getSelectedGroupPayload({
    ...nodeForm.value,
    GroupName: row.GroupNodes?.map((item) => item.Name) || [],
  });
  if (!group) return;
  await SetGroup({ name: row.Name, group });
}

function beginCreateSource() {
  editingSourceId.value = undefined;
  sourceForm.value = createEmptySource();
  rewriteRuleRows.value = [];
  sourceAdvanced.value = ["sync", "rules"];
}

function editSource(row: XUISource) {
  editingSourceId.value = row.id;
  sourceForm.value = {
    ...createEmptySource(),
    ...row,
    authType: row.authType === "apiToken" ? "apiToken" : "password",
    password: "",
    apiToken: "",
  };
  parseRewriteRules(row.rewriteRules || "");
  sourceAdvanced.value = ["sync", "rules"];
}

function normalizeSourcePayload() {
  const form = sourceForm.value;
  return {
    ...form,
    id: editingSourceId.value,
    authType: form.authType,
    host: form.authType === "password" ? form.host.trim() : "",
    username: form.authType === "password" ? form.username.trim() : "",
    password: form.authType === "password" ? (form.password || "").trim() : "",
    panelBaseUrl: form.authType === "apiToken" ? (form.panelBaseUrl || "").trim() : "",
    apiToken: form.authType === "apiToken" ? (form.apiToken || "").trim() : "",
    name: form.name.trim(),
    xuiDbPath: form.xuiDbPath.trim(),
    subBaseUrl: form.subBaseUrl.trim(),
    subPath: form.subPath.trim(),
    groupName: form.groupName.trim(),
    namePrefix: form.namePrefix.trim(),
    rewriteRules: serializeRewriteRules(),
  };
}

function validateSource() {
  const form = sourceForm.value;
  if (!form.name.trim()) return "请输入来源名称";
  if (form.authType === "password") {
    if (!form.host.trim()) return "请输入 SSH 主机";
    if (!form.username.trim()) return "请输入 SSH 用户名";
    if (!editingSourceId.value && !form.password?.trim()) return "请输入 SSH 密码";
  } else {
    if (!form.panelBaseUrl?.trim()) return "请输入面板地址";
    if (!editingSourceId.value && !form.apiToken?.trim()) return "请输入 API Token";
  }
  return "";
}

async function saveSource() {
  const errorText = validateSource();
  if (errorText) {
    ElMessage.warning(errorText);
    return;
  }

  sourceSaving.value = true;
  try {
    await SaveXUISource(normalizeSourcePayload());
    ElMessage.success(editingSourceId.value ? "来源已更新" : "来源已创建");
    beginCreateSource();
    await loadSources();
  } catch (error) {
    console.error(error);
    ElMessage.error("保存来源失败");
  } finally {
    sourceSaving.value = false;
  }
}

async function removeSource(row: XUISource) {
  if (!row.id) return;
  try {
    await ElMessageBox.confirm(`确定删除「${row.name}」吗？已同步的节点不会自动删除。`, "删除来源", {
      confirmButtonText: "删除",
      cancelButtonText: "取消",
      type: "warning",
    });
    await DelXUISource({ id: row.id });
    ElMessage.success("来源已删除");
    if (editingSourceId.value === row.id) beginCreateSource();
    await loadSources();
  } catch (error) {
    if (error !== "cancel") {
      console.error(error);
      ElMessage.error("删除来源失败");
    }
  }
}

async function syncOneSource(row: XUISource) {
  if (!row.id) return;
  try {
    const { data } = await SyncXUISource(row.id);
    showSyncResult(data || null);
    ElMessage.success(`${row.name}：${formatSyncResult(data)}`);
    await loadAll();
  } catch (error) {
    console.error(error);
    ElMessage.error(`${row.name} 同步失败`);
    await loadSources();
  }
}

async function syncAllSources() {
  syncingAllSources.value = true;
  try {
    await SyncAllXUISources();
    ElMessage.success("已同步全部启用来源");
    await loadAll();
  } catch (error) {
    console.error(error);
    ElMessage.error("同步全部来源失败");
  } finally {
    syncingAllSources.value = false;
  }
}

async function syncLocalXUI() {
  syncingLocalXUI.value = true;
  try {
    const { data } = await SyncXUINodes();
    showSyncResult(data || null);
    ElMessage.success(`本机 x-ui：${formatSyncResult(data)}`);
    await refreshNodeData();
  } catch (error) {
    console.error(error);
    ElMessage.error("同步本机 x-ui 失败");
  } finally {
    syncingLocalXUI.value = false;
  }
}

function showSyncResult(result: SyncResult | null) {
  clearSyncResultTimer();
  lastSyncResult.value = result;
  if (result) {
    syncResultTimer = window.setTimeout(() => {
      lastSyncResult.value = null;
      syncResultTimer = undefined;
    }, 6000);
  }
}

function clearSyncResultTimer() {
  if (syncResultTimer) {
    window.clearTimeout(syncResultTimer);
    syncResultTimer = undefined;
  }
}

function parseRewriteRules(raw: string) {
  rewriteRuleRows.value = [];
  const text = raw.trim();
  if (!text) return;
  try {
    const parsed = JSON.parse(text);
    const rules = Array.isArray(parsed) ? parsed : [parsed];
    rewriteRuleRows.value = rules.map((rule: any) => ({
      ...createRewriteRule(),
      nameContains: rule.nameContains || "",
      protocol: rule.protocol || "",
      transport: rule.transport || "",
      address: rule.address || "",
      port: rule.port || "",
      security: rule.security || "",
      sni: rule.sni || "",
      host: rule.host || "",
      fingerprint: rule.fp || rule.fingerprint || "",
      alpn:
        typeof rule.alpn === "string" && rule.alpn
          ? rule.alpn.split(",").map((item: string) => item.trim()).filter(Boolean)
          : [],
      path: rule.path || "",
      flow: rule.flow || "",
    }));
  } catch (error) {
    console.error(error);
    ElMessage.warning("改写规则解析失败，已忽略原规则");
  }
}

function serializeRewriteRules() {
  const rules = rewriteRuleRows.value
    .map((row) => {
      const rule: Record<string, string> = {};
      const setValue = (key: string, value?: string) => {
        const next = (value || "").trim();
        if (next) rule[key] = next;
      };
      setValue("nameContains", row.nameContains);
      setValue("protocol", row.protocol);
      setValue("transport", row.transport);
      setValue("address", row.address);
      setValue("port", row.port);
      setValue("security", row.security);
      setValue("sni", row.sni);
      setValue("host", row.host);
      setValue("fp", row.fingerprint);
      setValue("alpn", row.alpn.join(","));
      setValue("path", row.path);
      setValue("flow", row.flow);
      return rule;
    })
    .filter((rule) => Object.keys(rule).length > 0);
  return rules.length ? JSON.stringify(rules) : "";
}

function addRewriteRule() {
  if (!sourceAdvanced.value.includes("rules")) {
    sourceAdvanced.value = [...sourceAdvanced.value, "rules"];
  }
  rewriteRuleRows.value.push(createRewriteRule());
}

function removeRewriteRule(index: number) {
  rewriteRuleRows.value.splice(index, 1);
}

function formatDate(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function getGroupText(row: any) {
  if (!row.GroupNodes?.length) return "未分组";
  return row.GroupNodes.map((group: GroupNode) => group.Name).join(", ");
}

function getSourceAddress(row: XUISource) {
  return row.authType === "apiToken" ? row.panelBaseUrl || "-" : `${row.host}:${row.sshPort || 22}`;
}

function getStatusTag(row: XUISource) {
  if (row.lastSyncStatus === "success") return "success";
  if (row.lastSyncStatus === "failed") return "danger";
  return "info";
}

function getStatusText(row: XUISource) {
  if (row.lastSyncStatus === "success") return "成功";
  if (row.lastSyncStatus === "failed") return "失败";
  return "未同步";
}

function protocolFromLink(link: string) {
  const match = link.match(/^([a-z0-9+.-]+):\/\//i);
  return match ? match[1].toUpperCase() : "LINK";
}

function formatSyncResult(result?: SyncResult) {
  if (!result) return "同步完成";
  return `新增 ${result.created || 0}，更新 ${result.updated || 0}，不变 ${
    result.unchanged || 0
  }，跳过 ${result.skipped || 0}，删除 ${result.deleted || 0}`;
}
</script>

<template>
  <div class="node-page">
    <div class="node-toolbar">
      <div>
        <h2>节点管理</h2>
        <p>管理手动节点和远端 3x-ui / x-ui 来源，同步后会写入同一个节点池。</p>
      </div>
      <div class="toolbar-actions">
        <el-button :icon="Refresh" :loading="nodeLoading || sourceLoading" @click="loadAll">
          刷新
        </el-button>
        <el-button type="primary" :icon="Plus" @click="beginCreateNode">添加节点</el-button>
        <el-button type="success" :icon="Connection" @click="activePanel = 'sources'">
          远端 VPS 导入
        </el-button>
      </div>
    </div>

    <div class="summary-grid">
      <div class="summary-item">
        <span>节点总数</span>
        <strong>{{ nodeStats.total }}</strong>
      </div>
      <div class="summary-item">
        <span>分组</span>
        <strong>{{ nodeStats.groups }}</strong>
      </div>
      <div class="summary-item">
        <span>远端来源节点</span>
        <strong>{{ nodeStats.remote }}</strong>
      </div>
      <div class="summary-item">
        <span>启用来源</span>
        <strong>{{ sourceStats.enabled }}/{{ sourceStats.total }}</strong>
      </div>
    </div>

    <el-tabs v-model="activePanel" class="workspace-tabs">
      <el-tab-pane name="nodes">
        <template #label>
          <span class="tab-label"><el-icon><Connection /></el-icon>节点</span>
        </template>

        <section class="workspace-panel">
          <div class="panel-head">
            <div class="group-tabs">
              <el-button
                :type="activeGroup === ALL_GROUP_NAME ? 'primary' : 'default'"
                @click="activeGroup = ALL_GROUP_NAME"
              >
                全部 {{ nodes.length }}
              </el-button>
              <el-button
                v-for="group in groupNames"
                :key="group"
                :type="activeGroup === group ? 'primary' : 'default'"
                @click="activeGroup = group"
              >
                {{ group }}
              </el-button>
            </div>
            <el-input
              v-model="nodeKeyword"
              :prefix-icon="Search"
              clearable
              placeholder="搜索名称、链接、来源或 SubID"
              class="search-input"
            />
          </div>

          <div class="batch-bar">
            <span>已选 {{ nodeStats.selected }} 个</span>
            <el-button :icon="Check" @click="selectAllRows">全选当前</el-button>
            <el-button :icon="Close" @click="clearSelection">取消选择</el-button>
            <el-button type="primary" :icon="CopyDocument" @click="copySelectedNodes">
              复制选中
            </el-button>
            <el-button type="danger" :icon="Delete" @click="deleteSelectedNodes">
              删除选中
            </el-button>
          </div>

          <el-table
            ref="tableRef"
            v-loading="nodeLoading"
            :data="visibleNodes"
            row-key="ID"
            stripe
            class="node-table"
            empty-text="暂无节点"
            @selection-change="handleSelectionChange"
          >
            <el-table-column type="selection" width="46" />
            <el-table-column label="节点" min-width="230" sortable prop="Name">
              <template #default="{ row }">
                <div class="node-name">
                  <span>{{ row.Name }}</span>
                  <el-tag size="small" effect="plain">{{ protocolFromLink(row.Link) }}</el-tag>
                </div>
                <div class="node-meta">
                  {{ row.Source || "手动节点" }}
                  <template v-if="row.SubID"> · {{ row.SubID }}</template>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="链接" min-width="360" show-overflow-tooltip>
              <template #default="{ row }">
                <span class="link-text">{{ row.LinkOverride || row.Link }}</span>
              </template>
            </el-table-column>
            <el-table-column label="分组" min-width="160" show-overflow-tooltip>
              <template #default="{ row }">
                {{ getGroupText(row) }}
              </template>
            </el-table-column>
            <el-table-column label="创建时间" width="180" sortable prop="CreatedAt">
              <template #default="{ row }">
                {{ formatDate(row.CreatedAt || row.CreateDate) }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="176" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" :icon="EditPen" @click="beginEditNode(row)">
                  编辑
                </el-button>
                <el-button link type="primary" :icon="CopyDocument" @click="copyNode(row)">
                  复制
                </el-button>
                <el-button link type="danger" :icon="Delete" @click="deleteNode(row)">
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </section>
      </el-tab-pane>

      <el-tab-pane name="sources">
        <template #label>
          <span class="tab-label"><el-icon><Setting /></el-icon>远端 VPS 导入</span>
        </template>

        <section class="source-layout">
          <div class="source-list">
            <div class="panel-head compact">
              <div>
                <h3>远端 VPS</h3>
                <p>{{ sourceStats.enabled }} 个启用，{{ sourceStats.failed }} 个同步失败</p>
              </div>
              <el-button type="primary" :icon="Refresh" :loading="syncingAllSources" @click="syncAllSources">
                同步全部
              </el-button>
            </div>
            <el-input
              v-model="sourceKeyword"
              :prefix-icon="Search"
              clearable
                placeholder="搜索来源、地址、分组"
                class="source-search"
            />
            <div v-loading="sourceLoading" class="source-cards">
              <button
                v-for="source in sourceRows"
                :key="source.id"
                class="source-card"
                :class="{ active: editingSourceId === source.id, disabled: !source.enabled }"
                type="button"
                @click="editSource(source)"
              >
                <span class="source-card-title">
                  {{ source.name }}
                  <el-tag size="small" :type="source.enabled ? 'success' : 'info'">
                    {{ source.enabled ? "启用" : "停用" }}
                  </el-tag>
                </span>
                <span class="source-card-address">{{ getSourceAddress(source) }}</span>
                <span class="source-card-footer">
                  <el-tag size="small" :type="getStatusTag(source)">
                    {{ getStatusText(source) }}
                  </el-tag>
                  <span>{{ source.authType === "apiToken" ? "API Token" : "SSH 密码" }}</span>
                </span>
              </button>
              <el-empty v-if="sourceRows.length === 0" description="暂无来源" :image-size="80" />
            </div>
          </div>

          <div class="source-editor">
            <div class="panel-head compact">
              <div>
                <h3>{{ editingSourceId ? "编辑远端 VPS 导入" : "新增远端 VPS 导入" }}</h3>
                <p>通过 SSH 账号密码读取远端 3x-ui 数据库，或用 API Token 读取新版面板接口。</p>
              </div>
              <div class="source-head-actions">
                <el-button :icon="Plus" @click="beginCreateSource">新来源</el-button>
                <el-button type="primary" :icon="Plus" @click="addRewriteRule">
                  添加改写规则
                </el-button>
              </div>
            </div>

            <el-form :model="sourceForm" label-position="top" class="source-form">
              <div class="form-grid">
                <el-form-item label="来源名称">
                  <el-input v-model="sourceForm.name" placeholder="例如：香港 VPS / 洛杉矶 3x-ui" />
                </el-form-item>
                <el-form-item label="认证方式">
                  <el-segmented
                    v-model="sourceForm.authType"
                    :options="[
                      { label: 'SSH 账号密码', value: 'password' },
                      { label: 'API Token', value: 'apiToken' },
                    ]"
                  />
                </el-form-item>
                <el-form-item label="启用同步">
                  <el-switch v-model="sourceForm.enabled" />
                </el-form-item>
              </div>

              <div v-if="sourceForm.authType === 'password'" class="form-grid ssh-grid">
                <el-form-item label="SSH 主机">
                  <el-input v-model="sourceForm.host" placeholder="1.2.3.4 或 example.com" />
                </el-form-item>
                <el-form-item label="SSH 端口">
                  <el-input-number v-model="sourceForm.sshPort" :min="1" :max="65535" controls-position="right" />
                </el-form-item>
                <el-form-item label="用户名">
                  <el-input v-model="sourceForm.username" placeholder="root" />
                </el-form-item>
                <el-form-item label="密码">
                  <el-input
                    v-model="sourceForm.password"
                    type="password"
                    show-password
                    placeholder="编辑时留空保留原密码"
                  />
                </el-form-item>
              </div>

              <div v-else class="form-grid api-grid">
                <el-form-item label="面板地址">
                  <el-input v-model="sourceForm.panelBaseUrl" placeholder="https://panel.example.com:54321" />
                </el-form-item>
                <el-form-item label="API Token">
                  <el-input
                    v-model="sourceForm.apiToken"
                    type="password"
                    show-password
                    placeholder="编辑时留空保留原 Token"
                  />
                </el-form-item>
              </div>

              <el-collapse v-model="sourceAdvanced" class="source-collapse">
                <el-collapse-item title="同步设置" name="sync">
                  <div class="form-grid advanced-grid">
                    <el-form-item v-if="sourceForm.authType === 'password'" label="x-ui 数据库路径">
                      <el-input v-model="sourceForm.xuiDbPath" />
                    </el-form-item>
                    <el-form-item label="订阅 Base URL">
                      <el-input v-model="sourceForm.subBaseUrl" placeholder="留空时自动探测或保持原地址" />
                    </el-form-item>
                    <el-form-item label="订阅路径">
                      <el-input v-model="sourceForm.subPath" placeholder="例如 dingyue" />
                    </el-form-item>
                    <el-form-item label="导入分组">
                      <el-input v-model="sourceForm.groupName" placeholder="留空使用来源名称" />
                    </el-form-item>
                    <el-form-item label="节点名前缀">
                      <el-input v-model="sourceForm.namePrefix" placeholder="留空使用 [来源名称]" />
                    </el-form-item>
                    <el-form-item label="删除远端已缺失节点">
                      <el-switch v-model="sourceForm.deleteMissing" />
                    </el-form-item>
                  </div>
                </el-collapse-item>

                <el-collapse-item title="节点改写规则" name="rules">
                  <div class="rewrite-head">
                    <span>按名称、协议或传输方式匹配后，改写地址、端口、TLS、SNI、Path 等参数。</span>
                    <el-button size="small" type="primary" :icon="Plus" @click="addRewriteRule">
                      添加规则
                    </el-button>
                  </div>
                  <div v-if="rewriteRuleRows.length === 0" class="empty-rules">
                    暂无改写规则，同步时保持远端节点原始参数。
                  </div>
                  <div v-for="(rule, index) in rewriteRuleRows" :key="index" class="rewrite-rule">
                    <div class="rewrite-rule-title">
                      <span>规则 {{ index + 1 }}</span>
                      <el-button link type="danger" :icon="Delete" @click="removeRewriteRule(index)">
                        删除
                      </el-button>
                    </div>
                    <div class="form-grid rule-grid">
                      <el-form-item label="名称包含">
                        <el-input v-model="rule.nameContains" placeholder="可选" />
                      </el-form-item>
                      <el-form-item label="协议">
                        <el-select v-model="rule.protocol" clearable placeholder="不限">
                          <el-option label="VLESS" value="vless" />
                          <el-option label="VMess" value="vmess" />
                          <el-option label="Trojan" value="trojan" />
                        </el-select>
                      </el-form-item>
                      <el-form-item label="传输">
                        <el-select v-model="rule.transport" clearable placeholder="不限">
                          <el-option label="XHTTP" value="xhttp" />
                          <el-option label="TCP" value="tcp" />
                          <el-option label="WS" value="ws" />
                          <el-option label="gRPC" value="grpc" />
                        </el-select>
                      </el-form-item>
                      <el-form-item label="地址">
                        <el-input v-model="rule.address" placeholder="不填不改" />
                      </el-form-item>
                      <el-form-item label="端口">
                        <el-input v-model="rule.port" placeholder="不填不改" />
                      </el-form-item>
                      <el-form-item label="安全">
                        <el-select v-model="rule.security" clearable placeholder="不改">
                          <el-option label="TLS" value="tls" />
                          <el-option label="Reality" value="reality" />
                          <el-option label="None" value="none" />
                        </el-select>
                      </el-form-item>
                      <el-form-item label="SNI">
                        <el-input v-model="rule.sni" placeholder="不填不改" />
                      </el-form-item>
                      <el-form-item label="Host">
                        <el-input v-model="rule.host" placeholder="不填不改" />
                      </el-form-item>
                      <el-form-item label="Fingerprint">
                        <el-select v-model="rule.fingerprint" clearable placeholder="不改">
                          <el-option label="chrome" value="chrome" />
                          <el-option label="firefox" value="firefox" />
                          <el-option label="safari" value="safari" />
                          <el-option label="random" value="random" />
                        </el-select>
                      </el-form-item>
                      <el-form-item label="ALPN">
                        <el-select v-model="rule.alpn" multiple collapse-tags placeholder="不改">
                          <el-option label="h2" value="h2" />
                          <el-option label="http/1.1" value="http/1.1" />
                        </el-select>
                      </el-form-item>
                      <el-form-item label="Path">
                        <el-input v-model="rule.path" placeholder="不填不改" />
                      </el-form-item>
                      <el-form-item label="Flow">
                        <el-input v-model="rule.flow" placeholder="不填不改" />
                      </el-form-item>
                    </div>
                  </div>
                </el-collapse-item>
              </el-collapse>

              <div class="source-actions">
                <el-button type="primary" :icon="Check" :loading="sourceSaving" @click="saveSource">
                  保存来源
                </el-button>
                <el-button :icon="Close" @click="beginCreateSource">重置</el-button>
                <el-button
                  v-if="editingSourceId"
                  type="success"
                  :icon="Refresh"
                  @click="syncOneSource(sourceForm)"
                >
                  同步当前
                </el-button>
                <el-button
                  v-if="editingSourceId"
                  type="danger"
                  :icon="Delete"
                  @click="removeSource(sourceForm)"
                >
                  删除来源
                </el-button>
              </div>
            </el-form>

            <div class="local-sync">
              <div>
                <strong>本机 x-ui 数据库同步</strong>
                <span>保留旧入口，适合应用和 x-ui 在同一台机器时使用。</span>
              </div>
              <el-button :loading="syncingLocalXUI" :icon="Refresh" @click="syncLocalXUI">
                同步本机
              </el-button>
            </div>

            <el-alert
              v-if="lastSyncResult"
              :title="formatSyncResult(lastSyncResult)"
              type="success"
              show-icon
              :closable="false"
              class="sync-result"
            />
          </div>
        </section>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="nodeDialogVisible" :title="nodeDialogTitle" width="720px" class="node-dialog">
      <el-form :model="nodeForm" label-position="top">
        <el-form-item v-if="nodeDialogMode === 'edit'" label="节点名称">
          <el-input v-model="nodeForm.Name" placeholder="节点名称" />
        </el-form-item>
        <el-form-item label="节点链接">
          <el-input
            v-model="nodeForm.Link"
            type="textarea"
            :autosize="{ minRows: nodeDialogMode === 'add' ? 5 : 3, maxRows: 12 }"
            placeholder="支持多行或逗号分隔"
          />
        </el-form-item>
        <div class="form-grid">
          <el-form-item label="已有分组">
            <el-select v-model="nodeForm.GroupName" multiple clearable placeholder="可选">
              <el-option v-for="group in groupNames" :key="group" :label="group" :value="group" />
            </el-select>
          </el-form-item>
          <el-form-item label="新建分组">
            <el-input v-model="nodeForm.NewGroupName" placeholder="可选" />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="nodeDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="nodeSaving" @click="submitNodeForm">
          {{ nodeDialogMode === "add" ? "添加" : "保存" }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.node-page {
  min-height: 100%;
  padding: 20px;
  color: #1f2937;
  background:
    linear-gradient(180deg, rgba(239, 246, 255, 0.9), rgba(248, 250, 252, 0.4) 260px),
    #f6f8fb;
}

.node-toolbar,
.summary-item,
.workspace-panel,
.source-list,
.source-editor {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
}

.node-toolbar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  padding: 24px;
}

.node-toolbar h2,
.panel-head h3 {
  margin: 0;
  font-weight: 700;
  letter-spacing: 0;
  color: #111827;
}

.node-toolbar p,
.panel-head p {
  margin: 6px 0 0;
  color: #64748b;
}

.toolbar-actions,
.batch-bar,
.source-actions,
.source-head-actions,
.rewrite-head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(140px, 1fr));
  gap: 14px;
  margin: 16px 0;
}

.summary-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  position: relative;
  min-height: 76px;
  overflow: hidden;
  padding: 18px;
}

.summary-item::before {
  position: absolute;
  inset: 0 auto 0 0;
  width: 4px;
  content: "";
}

.summary-item:nth-child(1)::before {
  background: #2563eb;
}

.summary-item:nth-child(2)::before {
  background: #059669;
}

.summary-item:nth-child(3)::before {
  background: #d97706;
}

.summary-item:nth-child(4)::before {
  background: #e11d48;
}

.summary-item span {
  color: #64748b;
  font-size: 13px;
}

.summary-item strong {
  color: #111827;
  font-size: 30px;
  line-height: 1;
}

.workspace-tabs {
  margin-top: 8px;
}

.tab-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.workspace-panel,
.source-list,
.source-editor {
  padding: 18px;
}

.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.panel-head.compact {
  align-items: center;
}

.group-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.search-input {
  max-width: 340px;
}

.batch-bar {
  padding: 10px 12px;
  margin-bottom: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: rgba(248, 250, 252, 0.86);
}

.batch-bar span {
  margin-right: auto;
  color: #64748b;
}

.node-table {
  border: 1px solid #edf2f7;
  border-radius: 8px;
  overflow: hidden;
  background: #fff;
}

.node-name {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.node-name span {
  overflow: hidden;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-meta {
  margin-top: 4px;
  font-size: 12px;
  color: #64748b;
}

.link-text {
  color: #334155;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}

.source-layout {
  display: grid;
  grid-template-columns: minmax(280px, 340px) minmax(0, 1fr);
  gap: 14px;
}

.source-search {
  margin-bottom: 12px;
}

.source-cards {
  display: grid;
  gap: 10px;
  min-height: 220px;
}

.source-card {
  display: grid;
  gap: 8px;
  width: 100%;
  padding: 14px;
  text-align: left;
  cursor: pointer;
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  transition: border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease;
}

.source-card:hover,
.source-card.active {
  border-color: #0ea5e9;
  box-shadow: 0 10px 24px rgba(14, 165, 233, 0.12);
  transform: translateY(-1px);
}

.source-card.disabled {
  opacity: 0.68;
}

.source-card-title,
.source-card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.source-card-title {
  font-weight: 700;
}

.source-card-address,
.source-card-footer {
  font-size: 12px;
  color: #64748b;
}

.source-form {
  margin-top: 6px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.ssh-grid {
  grid-template-columns: minmax(0, 1.4fr) 150px minmax(0, 1fr) minmax(0, 1.4fr);
}

.api-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.advanced-grid,
.rule-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.source-collapse {
  margin-top: 4px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
}

.rewrite-head {
  justify-content: space-between;
  margin-bottom: 12px;
  color: #64748b;
}

.empty-rules {
  padding: 16px;
  color: #64748b;
  background: rgba(248, 250, 252, 0.86);
  border: 1px dashed #cbd5e1;
  border-radius: 8px;
}

.rewrite-rule {
  padding: 14px;
  margin-bottom: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: rgba(248, 250, 252, 0.72);
}

.rewrite-rule-title {
  display: flex;
  justify-content: space-between;
  margin-bottom: 10px;
  font-weight: 700;
}

.source-actions {
  padding-top: 4px;
}

.local-sync {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  margin-top: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: rgba(248, 250, 252, 0.86);
}

.local-sync div {
  display: grid;
  gap: 4px;
}

.local-sync span {
  color: #64748b;
}

.sync-result {
  margin-top: 12px;
}

:deep(.el-tabs__header) {
  margin-bottom: 12px;
}

:deep(.el-tabs__nav-wrap::after) {
  display: none;
}

:deep(.el-tabs__nav) {
  gap: 8px;
}

:deep(.el-tabs__item) {
  height: 40px;
  padding: 0 16px;
  border: 1px solid transparent;
  border-radius: 8px;
  color: #64748b;
  line-height: 40px;
}

:deep(.el-tabs__item.is-active) {
  border-color: #dbeafe;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
  color: #2563eb;
}

:deep(.el-tabs__active-bar) {
  display: none;
}

:deep(.el-collapse) {
  --el-collapse-header-height: 48px;
  overflow: hidden;
}

:deep(.el-collapse-item__header) {
  padding: 0 14px;
  border-bottom-color: #edf2f7;
  font-weight: 700;
}

:deep(.el-collapse-item__content) {
  padding: 14px;
}

:deep(.el-table th.el-table__cell) {
  background: #f8fafc;
  color: #475569;
}

:deep(.el-form-item) {
  margin-bottom: 16px;
}

:deep(.el-segmented) {
  --el-segmented-item-selected-bg-color: var(--el-color-primary);
  --el-segmented-item-selected-color: #fff;
}

@media (max-width: 1180px) {
  .source-layout,
  .summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .source-list,
  .source-editor {
    grid-column: 1 / -1;
  }

  .form-grid,
  .ssh-grid,
  .api-grid,
  .advanced-grid,
  .rule-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .node-page {
    padding: 10px;
  }

  .node-toolbar,
  .panel-head,
  .local-sync {
    align-items: stretch;
    flex-direction: column;
  }

  .summary-grid,
  .source-layout,
  .form-grid,
  .ssh-grid,
  .api-grid,
  .advanced-grid,
  .rule-grid {
    grid-template-columns: 1fr;
  }

  .search-input {
    max-width: none;
  }
}
</style>
