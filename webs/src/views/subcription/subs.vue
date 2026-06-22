<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
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
const selectedNodeNames = ref<string[]>([]);
const logs = ref<SubLog[]>([]);
const subscriptionDialogVisible = ref(false);
const clientDialogVisible = ref(false);
const qrDialogVisible = ref(false);
const logsDialogVisible = ref(false);
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

const pagedSubscriptions = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value;
  return subscriptions.value.slice(start, start + pageSize.value);
});

onMounted(async () => {
  await Promise.all([loadSubscriptions(), loadNodes(), loadTemplates()]);
});

async function loadSubscriptions() {
  const { data } = await getSubs();
  subscriptions.value = Array.isArray(data) ? data : [];
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
  dialogTitle.value = "添加订阅";
  subName.value = "";
  subToken.value = generateSubscriptionToken();
  oldSubName.value = "";
  selectedNodeNames.value = [];
  enabledOptions.value = ["udp"];
  clashTemplate.value = "./template/clash.yaml";
  surgeTemplate.value = "./template/surge.conf";
  clashTemplateMode.value = "local";
  surgeTemplateMode.value = "local";
  subscriptionDialogVisible.value = true;
}

function openEditDialog(row: Sub | any) {
  const config = parseConfig(row.Config);
  dialogTitle.value = "编辑订阅";
  subName.value = row.Name;
  subToken.value = getSubscriptionToken(row);
  oldSubName.value = row.Name;
  selectedNodeNames.value = row.Nodes?.map((item: Node) => item.Name) ?? [];
  enabledOptions.value = [];
  if (config.udp) enabledOptions.value.push("udp");
  if (config.cert) enabledOptions.value.push("cert");
  clashTemplate.value = config.clash || "./template/clash.yaml";
  surgeTemplate.value = config.surge || "./template/surge.conf";
  clashTemplateMode.value = clashTemplate.value.startsWith("http") ? "url" : "local";
  surgeTemplateMode.value = surgeTemplate.value.startsWith("http") ? "url" : "local";
  subscriptionDialogVisible.value = true;
}

async function persistSubscriptionOrder(row: Sub | any) {
  const nodeNames = row.Nodes?.map((item: Node) => item.Name).filter(Boolean) ?? [];
  if (!row.Name || !nodeNames.length) return;

  try {
    await UpdateSub({
      config: typeof row.Config === "string" ? row.Config : JSON.stringify(row.Config),
      name: row.Name,
      oldname: row.Name,
      token: getSubscriptionToken(row),
      nodes: nodeNames.join(","),
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
  if (!selectedNodeNames.value.length) {
    ElMessage.warning("请至少选择一个节点");
    return;
  }
  const token = subToken.value.trim().toLowerCase();
  if (!isValidSubscriptionToken(token)) {
    ElMessage.warning("订阅链接标识只能包含 6-64 位小写字母、数字、下划线和短横线");
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
        nodes: selectedNodeNames.value.join(","),
      });
      ElMessage.success("订阅已添加");
    } else {
      await UpdateSub({
        config: JSON.stringify(config),
        name: subName.value.trim(),
        oldname: oldSubName.value,
        token,
        nodes: selectedNodeNames.value.join(","),
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
    await ElMessageBox.confirm(`确定删除选中的 ${selectedRows.value.length} 个订阅吗？`, "批量删除", {
      confirmButtonText: "删除",
      cancelButtonText: "取消",
      type: "warning",
    });
    await Promise.all(selectedRows.value.map((item) => DelSub({ id: item.ID })));
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
  clientUrls.value = clientOptions.reduce<Record<string, string>>((result, option) => {
    result[option.key] = option.param ? `${baseUrl}&client=${option.param}` : baseUrl;
    return result;
  }, {});
  clientDialogVisible.value = true;
}

function getSubscriptionToken(row: Sub | any) {
  return String(row.Token || row.token || md5(row.Name)).trim().toLowerCase();
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
        <h2>订阅管理</h2>
        <p>为 Clash Verge Rev / Mihomo、Surge 和 V2Ray 生成订阅地址。</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openAddDialog">添加订阅</el-button>
    </section>

    <el-card shadow="never" class="content-card">
      <el-table
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
                <el-tag v-if="row.Nodes?.length" effect="plain" round>可排序</el-tag>
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
                <div v-for="(node, index) in row.Nodes" :key="node.ID || node.Name" class="expanded-node-item">
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
              <span class="token-line">标识：{{ getSubscriptionToken(row) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="节点数量" width="110">
          <template #default="{ row }">
            <el-tag :type="getNodeCount(row) ? 'primary' : 'info'" effect="light" round>
              {{ formatNodeCount(row) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="客户端入口" min-width="180">
          <template #default="{ row }">
            <el-button link type="primary" :icon="Link" @click="showClientLinks(row)">
              订阅地址
            </el-button>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="180" :formatter="formatDate" sortable />
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :icon="Tickets" @click="showLogs(row)">记录</el-button>
            <el-button link type="primary" :icon="Edit" @click="openEditDialog(row)">编辑</el-button>
            <el-button link type="danger" :icon="Delete" @click="deleteSubscription(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="table-footer">
        <div class="batch-actions">
          <el-button type="danger" :icon="Delete" @click="deleteSelected">删除选中</el-button>
        </div>
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 30, 40]"
          :total="subscriptions.length"
          layout="total, sizes, prev, pager, next, jumper"
        />
      </div>
    </el-card>

    <el-dialog v-model="subscriptionDialogVisible" :title="dialogTitle" width="760px">
      <el-form label-position="top">
        <el-form-item label="订阅名称">
          <el-input v-model="subName" placeholder="例如：全部节点" />
        </el-form-item>

        <el-form-item label="订阅链接标识">
          <div class="token-editor">
            <el-input
              v-model="subToken"
              maxlength="64"
              placeholder="用于生成订阅链接，可手动修改"
              @input="subToken = subToken.trim().toLowerCase()"
            />
            <el-button @click="resetSubscriptionToken">生成新链接</el-button>
          </div>
          <div class="sort-helper">
            修改后新的客户端订阅地址会变化，旧地址将不再指向这个订阅。
          </div>
        </el-form-item>

        <el-form-item label="Clash / Mihomo 模板">
          <el-radio-group v-model="clashTemplateMode">
            <el-radio label="local">本地模板</el-radio>
            <el-radio label="url">URL 模板</el-radio>
          </el-radio-group>
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

        <el-form-item label="Surge 模板">
          <el-radio-group v-model="surgeTemplateMode">
            <el-radio label="local">本地模板</el-radio>
            <el-radio label="url">URL 模板</el-radio>
          </el-radio-group>
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

        <el-form-item label="输出选项">
          <el-checkbox-group v-model="enabledOptions">
            <el-checkbox label="udp">启用 UDP</el-checkbox>
            <el-checkbox label="cert">跳过证书校验</el-checkbox>
          </el-checkbox-group>
        </el-form-item>

        <el-form-item label="选择节点">
          <el-select
            v-model="selectedNodeNames"
            multiple
            filterable
            collapse-tags
            collapse-tags-tooltip
            class="full-width"
            placeholder="选择要包含的节点"
          >
            <el-option v-for="item in nodes" :key="item.Name" :label="item.Name" :value="item.Name" />
          </el-select>
        </el-form-item>

        <el-form-item v-if="selectedNodeNames.length" label="节点排序">
          <div class="sort-helper">拖动左侧手柄调整顺序，保存后会写入订阅的节点输出顺序。</div>
          <VueDraggable
            v-model="selectedNodeNames"
            :animation="160"
            ghost-class="ghost"
            handle=".drag-handle"
            class="node-order"
          >
            <div v-for="(nodeName, index) in selectedNodeNames" :key="nodeName" class="draggable-item">
              <el-icon class="drag-handle"><Rank /></el-icon>
              <span class="row-number">{{ index + 1 }}</span>
              <span class="node-title">{{ nodeName }}</span>
            </div>
          </VueDraggable>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="subscriptionDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitSubscription">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="clientDialogVisible" title="客户端订阅地址" width="680px">
      <div class="client-list">
        <div v-for="option in clientOptions" :key="option.key" class="client-row">
          <div>
            <strong>{{ option.label }}</strong>
            <p>{{ clientUrls[option.key] }}</p>
          </div>
          <div class="client-actions">
            <el-button :icon="CopyDocument" @click="copyText(clientUrls[option.key])">复制</el-button>
            <el-button @click="showQr(option.label, clientUrls[option.key])">二维码</el-button>
            <el-button type="primary" @click="openUrl(clientUrls[option.key])">打开</el-button>
          </div>
        </div>
      </div>
    </el-dialog>

    <el-dialog v-model="qrDialogVisible" :title="qrTitle" width="360px" class="qr-dialog">
      <div class="qr-box">
        <qrcode-vue :value="qrUrl" :size="220" level="H" />
        <el-input v-model="qrUrl" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" />
        <div class="client-actions">
          <el-button :icon="CopyDocument" @click="copyText(qrUrl)">复制</el-button>
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
  color: #1f2937;
  background:
    linear-gradient(180deg, rgba(239, 246, 255, 0.9), rgba(248, 250, 252, 0.4) 260px),
    #f6f8fb;
}

.page-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  padding: 24px;
  margin-bottom: 16px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
}

.page-header h2 {
  margin: 0 0 6px;
  color: #111827;
  font-size: 26px;
  font-weight: 750;
  letter-spacing: 0;
}

.page-header p {
  margin: 0;
  color: #64748b;
}

.content-card {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
}

.content-card :deep(.el-card__body) {
  padding: 18px;
}

.content-card :deep(.el-table) {
  overflow: hidden;
  border: 1px solid #edf2f7;
  border-radius: 8px;
}

.content-card :deep(.el-table th.el-table__cell) {
  background: #f8fafc;
  color: #475569;
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
  background: #f8fafc;
}

.expanded-node-panel {
  margin: 12px 18px 18px;
  padding: 16px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: linear-gradient(180deg, #ffffff, #f8fafc);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.8);
}

.expanded-node-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
  margin-bottom: 12px;
  border-bottom: 1px solid #e5e7eb;
}

.expanded-node-header strong {
  display: block;
  margin-bottom: 3px;
  color: #111827;
  font-size: 15px;
}

.expanded-node-header span {
  color: #64748b;
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
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #ffffff;
  transition: border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease;
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
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
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

.node-order {
  width: 100%;
}

.sort-helper {
  width: 100%;
  margin: -2px 0 10px;
  color: #64748b;
  font-size: 13px;
}

.draggable-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  margin-bottom: 8px;
  cursor: grab;
  background: rgba(248, 250, 252, 0.86);
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  transition: border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease;
}

.draggable-item:hover,
.expanded-node-item:hover {
  border-color: #bfdbfe;
  box-shadow: 0 8px 20px rgba(37, 99, 235, 0.1);
  transform: translateY(-1px);
}

.drag-handle {
  flex: 0 0 auto;
  color: #94a3b8;
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
  color: #475569;
  background: #e2e8f0;
  border-radius: 8px;
}

.row-number.compact {
  width: 22px;
  height: 22px;
}

.node-title {
  min-width: 0;
  overflow: hidden;
  color: #111827;
  font-weight: 650;
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
  color: #94a3b8;
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
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: rgba(248, 250, 252, 0.72);
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

@media (max-width: 760px) {
  .page-header,
  .table-footer,
  .client-row,
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
  .token-editor .el-button {
    margin-top: 12px;
  }
}
</style>
