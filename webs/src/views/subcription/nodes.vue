<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from "vue";
import {
  CircleCheck,
  CopyDocument,
  Delete,
  Edit,
  Files,
  FolderOpened,
  Plus,
  Refresh,
  Search,
} from "@element-plus/icons-vue";
import {
  AddNodes,
  DelNode,
  GetGroup,
  SetGroup,
  UpdateNode,
  getNodes,
} from "@/api/subcription/node";

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

type DialogMode = "add" | "edit";

const loading = ref(false);
const saving = ref(false);
const activeGroup = ref("全部");
const protocolFilter = ref("全部");
const searchKeyword = ref("");
const nodes = ref<NodeItem[]>([]);
const groups = ref<string[]>([]);
const selectedRows = ref<NodeItem[]>([]);
const tableRef = ref<any>(null);

const nodeDialogVisible = ref(false);
const groupDialogVisible = ref(false);
const dialogMode = ref<DialogMode>("add");
const nodeForm = reactive({
  ID: 0,
  Name: "",
  Link: "",
  selectedGroups: [] as string[],
  newGroups: "",
});
const groupForm = reactive({
  nodeName: "",
  selectedGroups: [] as string[],
  newGroups: "",
});

const protocolOptions = computed(() => {
  const options = new Set<string>(["全部"]);
  nodes.value.forEach((item) => options.add(getProtocolLabel(item.Link)));
  return Array.from(options);
});

const groupCounts = computed(() => {
  const map = new Map<string, number>();
  groups.value.forEach((item) => map.set(item, 0));
  nodes.value.forEach((node) => {
    node.GroupNodes?.forEach((group) => {
      map.set(group.Name, (map.get(group.Name) ?? 0) + 1);
    });
  });
  return map;
});

const groupedNodeCount = computed(
  () => nodes.value.filter((item) => (item.GroupNodes?.length ?? 0) > 0).length
);
const xhttpCount = computed(() => nodes.value.filter((item) => isXhttpNode(item.Link)).length);
const filteredNodes = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase();
  return nodes.value.filter((item) => {
    const groupMatch =
      activeGroup.value === "全部" ||
      item.GroupNodes?.some((group) => group.Name === activeGroup.value);
    const protocolMatch =
      protocolFilter.value === "全部" || getProtocolLabel(item.Link) === protocolFilter.value;
    const keywordMatch =
      !keyword ||
      item.Name.toLowerCase().includes(keyword) ||
      item.Link.toLowerCase().includes(keyword) ||
      getGroupNames(item).join(" ").toLowerCase().includes(keyword);
    return groupMatch && protocolMatch && keywordMatch;
  });
});

const metricCards = computed(() => [
  { label: "全部节点", value: nodes.value.length, icon: Files, tone: "blue" },
  { label: "已分组", value: groupedNodeCount.value, icon: FolderOpened, tone: "green" },
  { label: "xhttp", value: xhttpCount.value, icon: CircleCheck, tone: "amber" },
  { label: "已选择", value: selectedRows.value.length, icon: CopyDocument, tone: "rose" },
]);

onMounted(() => {
  refreshData();
});

async function refreshData() {
  loading.value = true;
  try {
    const [nodeResult, groupResult] = await Promise.all([getNodes(), GetGroup()]);
    nodes.value = Array.isArray(nodeResult.data) ? nodeResult.data : [];
    groups.value = Array.isArray(groupResult.data) ? groupResult.data : [];
    if (activeGroup.value !== "全部" && !groups.value.includes(activeGroup.value)) {
      activeGroup.value = "全部";
    }
  } finally {
    loading.value = false;
  }
}

function openAddDialog() {
  dialogMode.value = "add";
  Object.assign(nodeForm, {
    ID: 0,
    Name: "",
    Link: "",
    selectedGroups: activeGroup.value === "全部" ? [] : [activeGroup.value],
    newGroups: "",
  });
  nodeDialogVisible.value = true;
}

function openEditDialog(row: NodeItem | any) {
  dialogMode.value = "edit";
  Object.assign(nodeForm, {
    ID: row.ID,
    Name: row.Name,
    Link: row.Link,
    selectedGroups: getGroupNames(row),
    newGroups: "",
  });
  nodeDialogVisible.value = true;
}

async function submitNode() {
  const links = splitNodeLinks(nodeForm.Link);
  const group = collectGroups(nodeForm.selectedGroups, nodeForm.newGroups).join(",");
  if (dialogMode.value === "add" && !links.length) {
    ElMessage.warning("请至少输入一个节点链接");
    return;
  }
  if (dialogMode.value === "edit") {
    if (!nodeForm.Name.trim()) {
      ElMessage.warning("节点名称不能为空");
      return;
    }
    if (!nodeForm.Link.trim()) {
      ElMessage.warning("节点链接不能为空");
      return;
    }
  }

  saving.value = true;
  try {
    if (dialogMode.value === "add") {
      await Promise.all(links.map((link) => AddNodes({ link, group })));
      ElMessage.success(`已导入 ${links.length} 个节点`);
    } else {
      await UpdateNode({
        id: nodeForm.ID,
        name: nodeForm.Name.trim(),
        link: nodeForm.Link.trim(),
        group,
      });
      ElMessage.success("节点已更新");
    }
    nodeDialogVisible.value = false;
    await refreshData();
  } finally {
    saving.value = false;
  }
}

function openGroupDialog(row?: NodeItem | any) {
  const target = row ?? selectedRows.value[0];
  Object.assign(groupForm, {
    nodeName: target?.Name ?? "",
    selectedGroups: target ? getGroupNames(target) : activeGroup.value === "全部" ? [] : [activeGroup.value],
    newGroups: "",
  });
  groupDialogVisible.value = true;
}

function syncGroupForm() {
  const target = nodes.value.find((item) => item.Name === groupForm.nodeName);
  groupForm.selectedGroups = target ? getGroupNames(target) : [];
}

async function submitGroupBinding() {
  if (!groupForm.nodeName) {
    ElMessage.warning("请先选择节点");
    return;
  }
  const group = collectGroups(groupForm.selectedGroups, groupForm.newGroups).join(",");
  if (!group) {
    ElMessage.warning("请至少选择或输入一个分组");
    return;
  }

  saving.value = true;
  try {
    await SetGroup({ name: groupForm.nodeName, group });
    ElMessage.success("分组已更新");
    groupDialogVisible.value = false;
    await refreshData();
  } finally {
    saving.value = false;
  }
}

async function deleteNode(row: NodeItem | any) {
  await ElMessageBox.confirm(`确定删除「${row.Name}」吗？`, "删除节点", {
    confirmButtonText: "删除",
    cancelButtonText: "取消",
    type: "warning",
  });
  await DelNode({ id: row.ID });
  ElMessage.success("节点已删除");
  await refreshData();
}

async function deleteSelected() {
  if (!selectedRows.value.length) {
    ElMessage.warning("请先选择节点");
    return;
  }
  await ElMessageBox.confirm(`确定删除选中的 ${selectedRows.value.length} 个节点吗？`, "批量删除", {
    confirmButtonText: "删除",
    cancelButtonText: "取消",
    type: "warning",
  });
  await Promise.all(selectedRows.value.map((item) => DelNode({ id: item.ID })));
  selectedRows.value = [];
  ElMessage.success("已删除选中节点");
  await refreshData();
}

function selectCurrentPage() {
  nextTick(() => {
    filteredNodes.value.forEach((row) => tableRef.value?.toggleRowSelection(row, true));
  });
}

function clearSelection() {
  tableRef.value?.clearSelection();
  selectedRows.value = [];
}

function handleSelectionChange(selection: NodeItem[]) {
  selectedRows.value = selection;
}

async function copyNode(row: NodeItem | any) {
  await copyText(row.Link);
}

async function copySelected() {
  if (!selectedRows.value.length) {
    ElMessage.warning("请先选择节点");
    return;
  }
  await copyText(selectedRows.value.map((item) => item.Link).join("\n"));
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

function splitNodeLinks(value: string) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function collectGroups(selected: string[], extra: string) {
  const next = new Set<string>();
  selected.forEach((item) => item.trim() && next.add(item.trim()));
  extra
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
    .forEach((item) => next.add(item));
  return Array.from(next);
}

function getGroupNames(row: NodeItem | any): string[] {
  return row.GroupNodes?.map((item: GroupNode) => item.Name).filter(Boolean) ?? [];
}

function getProtocol(link: string) {
  return link.split("://")[0]?.trim().toUpperCase() || "UNKNOWN";
}

function getProtocolLabel(link: string) {
  if (isXhttpNode(link)) return "VLESS xhttp";
  return getProtocol(link);
}

function decodeLinkBody(link: string) {
  const raw = link.split("://")[1] ?? "";
  try {
    return decodeURIComponent(atob(raw));
  } catch {
    try {
      return decodeURIComponent(link);
    } catch {
      return link;
    }
  }
}

function isXhttpNode(link: string) {
  return decodeLinkBody(link).toLowerCase().includes("type=xhttp");
}

function formatDate(row: NodeItem | any) {
  const value = row.CreatedAt ?? row.CreateDate;
  return value ? new Date(value).toLocaleString() : "-";
}
</script>

<template>
  <div class="node-page">
    <section class="page-hero">
      <div>
        <span class="eyebrow">节点管理</span>
        <h1>整理节点、分组和 xhttp 支持</h1>
        <p>导入节点后可按协议、分组和关键字筛选。VLESS xhttp 会在 Clash Verge Rev / Mihomo 配置中生成 xhttp-opts。</p>
      </div>
      <div class="hero-actions">
        <el-button :icon="Refresh" @click="refreshData">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="openAddDialog">导入节点</el-button>
      </div>
    </section>

    <section class="metric-grid">
      <div v-for="item in metricCards" :key="item.label" class="metric-card" :class="`tone-${item.tone}`">
        <div class="metric-icon">
          <el-icon><component :is="item.icon" /></el-icon>
        </div>
        <div>
          <span>{{ item.label }}</span>
          <strong>{{ item.value }}</strong>
        </div>
      </div>
    </section>

    <section class="workspace-panel">
      <aside class="group-sidebar">
        <div class="sidebar-title">分组</div>
        <button :class="{ active: activeGroup === '全部' }" @click="activeGroup = '全部'">
          <span>全部</span>
          <strong>{{ nodes.length }}</strong>
        </button>
        <button
          v-for="group in groups"
          :key="group"
          :class="{ active: activeGroup === group }"
          @click="activeGroup = group"
        >
          <span>{{ group }}</span>
          <strong>{{ groupCounts.get(group) ?? 0 }}</strong>
        </button>
      </aside>

      <main class="table-panel">
        <div class="toolbar">
          <el-input
            v-model="searchKeyword"
            :prefix-icon="Search"
            clearable
            placeholder="搜索节点名称、链接或分组"
            class="search-input"
          />
          <el-select v-model="protocolFilter" class="protocol-select" placeholder="协议">
            <el-option v-for="item in protocolOptions" :key="item" :label="item" :value="item" />
          </el-select>
          <div class="toolbar-actions">
            <el-button :icon="CopyDocument" @click="copySelected">复制选中</el-button>
            <el-button :icon="FolderOpened" @click="openGroupDialog()">绑定分组</el-button>
            <el-button type="danger" :icon="Delete" @click="deleteSelected">删除选中</el-button>
          </div>
        </div>

        <el-table
          ref="tableRef"
          v-loading="loading"
          :data="filteredNodes"
          row-key="ID"
          height="calc(100vh - 390px)"
          @selection-change="handleSelectionChange"
        >
          <el-table-column type="selection" width="44" />
          <el-table-column label="节点" min-width="240" sortable prop="Name">
            <template #default="{ row }">
              <div class="node-name">
                <strong>{{ row.Name }}</strong>
                <span>{{ getProtocolLabel(row.Link) }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="分组" min-width="180">
            <template #default="{ row }">
              <div class="tag-list" v-if="getGroupNames(row).length">
                <el-tag v-for="group in getGroupNames(row)" :key="group" effect="plain" round>{{ group }}</el-tag>
              </div>
              <span v-else class="muted">未分组</span>
            </template>
          </el-table-column>
          <el-table-column label="节点链接" min-width="360" show-overflow-tooltip>
            <template #default="{ row }">
              <code class="node-link">{{ row.Link }}</code>
            </template>
          </el-table-column>
          <el-table-column label="创建时间" width="180" :formatter="formatDate" />
          <el-table-column label="操作" width="230" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" :icon="Edit" @click="openEditDialog(row)">编辑</el-button>
              <el-button link type="primary" :icon="CopyDocument" @click="copyNode(row)">复制</el-button>
              <el-button link type="primary" :icon="FolderOpened" @click="openGroupDialog(row)">分组</el-button>
              <el-button link type="danger" :icon="Delete" @click="deleteNode(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="table-footer">
          <span>当前显示 {{ filteredNodes.length }} 个节点，已选择 {{ selectedRows.length }} 个</span>
          <div>
            <el-button link type="primary" @click="selectCurrentPage">选择当前结果</el-button>
            <el-button link @click="clearSelection">清空选择</el-button>
          </div>
        </div>
      </main>
    </section>

    <el-dialog v-model="nodeDialogVisible" :title="dialogMode === 'add' ? '导入节点' : '编辑节点'" width="720px">
      <el-form label-position="top">
        <el-form-item v-if="dialogMode === 'edit'" label="节点名称">
          <el-input v-model="nodeForm.Name" placeholder="节点名称" />
        </el-form-item>
        <el-form-item :label="dialogMode === 'add' ? '节点链接' : '节点链接'">
          <el-input
            v-model="nodeForm.Link"
            type="textarea"
            :autosize="{ minRows: 5, maxRows: 12 }"
            placeholder="支持多行或英文逗号分隔。添加时会逐条导入，编辑时只保存当前节点。"
          />
        </el-form-item>
        <el-form-item label="分组">
          <el-select
            v-model="nodeForm.selectedGroups"
            multiple
            filterable
            clearable
            class="full-width"
            placeholder="选择已有分组"
          >
            <el-option v-for="group in groups" :key="group" :label="group" :value="group" />
          </el-select>
          <el-input
            v-model="nodeForm.newGroups"
            class="full-width"
            placeholder="新分组，可用逗号或换行分隔"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="nodeDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitNode">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="groupDialogVisible" title="绑定分组" width="560px">
      <el-form label-position="top">
        <el-form-item label="节点">
          <el-select
            v-model="groupForm.nodeName"
            filterable
            class="full-width"
            placeholder="选择节点"
            @change="syncGroupForm"
          >
            <el-option v-for="item in nodes" :key="item.ID" :label="item.Name" :value="item.Name" />
          </el-select>
        </el-form-item>
        <el-form-item label="分组">
          <el-select
            v-model="groupForm.selectedGroups"
            multiple
            filterable
            clearable
            class="full-width"
            placeholder="选择已有分组"
          >
            <el-option v-for="group in groups" :key="group" :label="group" :value="group" />
          </el-select>
          <el-input
            v-model="groupForm.newGroups"
            class="full-width"
            placeholder="新分组，可用逗号或换行分隔"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitGroupBinding">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.node-page {
  min-height: 100%;
  padding: 18px;
  color: #1f2937;
  background:
    linear-gradient(180deg, rgba(240, 249, 255, 0.95), rgba(248, 250, 252, 0.5) 280px),
    #f6f8fb;
}

.page-hero,
.workspace-panel,
.metric-card {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
}

.page-hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 18px;
  padding: 22px 24px;
}

.eyebrow {
  color: #2563eb;
  font-size: 13px;
  font-weight: 750;
}

.page-hero h1 {
  margin: 8px 0 6px;
  font-size: 25px;
  font-weight: 760;
}

.page-hero p {
  max-width: 760px;
  margin: 0;
  color: #64748b;
}

.hero-actions,
.toolbar-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin: 14px 0;
}

.metric-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 15px;
}

.metric-card span {
  color: #64748b;
  font-size: 13px;
}

.metric-card strong {
  display: block;
  margin-top: 2px;
  color: #111827;
  font-size: 26px;
  line-height: 1;
}

.metric-icon {
  display: grid;
  width: 40px;
  height: 40px;
  place-items: center;
  border-radius: 8px;
  font-size: 21px;
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

.workspace-panel {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  min-height: calc(100vh - 270px);
  overflow: hidden;
}

.group-sidebar {
  padding: 16px;
  border-right: 1px solid #e5e7eb;
  background: #fbfdff;
}

.sidebar-title {
  margin-bottom: 10px;
  color: #64748b;
  font-size: 13px;
  font-weight: 750;
}

.group-sidebar button {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
  padding: 10px 12px;
  border: 0;
  border-radius: 8px;
  color: #334155;
  background: transparent;
  cursor: pointer;
  text-align: left;
}

.group-sidebar button:hover {
  background: #eef6ff;
}

.group-sidebar button.active {
  color: #1d4ed8;
  background: #dbeafe;
  font-weight: 700;
}

.group-sidebar strong {
  font-size: 12px;
}

.table-panel {
  min-width: 0;
  padding: 16px;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
}

.search-input {
  max-width: 420px;
}

.protocol-select {
  width: 160px;
}

.toolbar-actions {
  margin-left: auto;
}

.node-name strong {
  display: block;
  color: #111827;
}

.node-name span,
.muted {
  color: #64748b;
  font-size: 12px;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.node-link {
  color: #334155;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 12px;
}

.table-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 12px;
  color: #64748b;
}

.full-width {
  width: 100%;
  margin-top: 8px;
}

@media (max-width: 1100px) {
  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .workspace-panel {
    grid-template-columns: 1fr;
  }

  .group-sidebar {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    border-right: 0;
    border-bottom: 1px solid #e5e7eb;
  }

  .sidebar-title {
    width: 100%;
    margin-bottom: 0;
  }

  .group-sidebar button {
    width: auto;
    margin-bottom: 0;
  }
}

@media (max-width: 760px) {
  .node-page {
    padding: 12px;
  }

  .page-hero,
  .toolbar,
  .table-footer {
    display: block;
  }

  .hero-actions,
  .toolbar-actions {
    margin-top: 14px;
  }

  .metric-grid {
    grid-template-columns: 1fr;
  }

  .search-input,
  .protocol-select {
    width: 100%;
    max-width: none;
    margin-bottom: 8px;
  }
}
</style>
