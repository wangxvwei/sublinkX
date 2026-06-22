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
  Tickets,
} from "@element-plus/icons-vue";
import { AddSub, DelSub, UpdateSub, getSubs } from "@/api/subcription/subs";
import { getNodes } from "@/api/subcription/node";
import { getTemp } from "@/api/subcription/temp";

interface Sub {
  ID: number;
  Name: string;
  Config: string | Config;
  Nodes: Node[];
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

async function submitSubscription() {
  if (!subName.value.trim()) {
    ElMessage.warning("订阅名称不能为空");
    return;
  }
  if (!selectedNodeNames.value.length) {
    ElMessage.warning("请至少选择一个节点");
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
        nodes: selectedNodeNames.value.join(","),
      });
      ElMessage.success("订阅已添加");
    } else {
      await UpdateSub({
        config: JSON.stringify(config),
        name: subName.value.trim(),
        oldname: oldSubName.value,
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

function showLogs(row: Sub | any) {
  logs.value = row.SubLogs ?? [];
  logsDialogVisible.value = true;
}

function showClientLinks(row: Sub | any) {
  const baseUrl = `${location.protocol}//${location.host}/c/?token=${md5(row.Name)}`;
  clientUrls.value = clientOptions.reduce<Record<string, string>>((result, option) => {
    result[option.key] = option.param ? `${baseUrl}&client=${option.param}` : baseUrl;
    return result;
  }, {});
  clientDialogVisible.value = true;
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
        <el-table-column prop="Name" label="订阅名称" min-width="180">
          <template #default="{ row }">
            <el-tag effect="plain">{{ row.Name }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="节点数量" width="110">
          <template #default="{ row }">
            {{ row.Nodes?.length ?? 0 }}
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
          <VueDraggable v-model="selectedNodeNames" :animation="150" ghost-class="ghost" class="node-order">
            <div v-for="(nodeName, index) in selectedNodeNames" :key="nodeName" class="draggable-item">
              <span class="row-number">{{ index + 1 }}</span>
              <span>{{ nodeName }}</span>
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
  padding: 16px;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.page-header h2 {
  margin: 0 0 6px;
  font-size: 22px;
  font-weight: 650;
}

.page-header p {
  margin: 0;
  color: var(--el-text-color-secondary);
}

.content-card {
  border-radius: 6px;
}

.table-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 16px;
}

.full-width {
  width: 100%;
  margin-top: 10px;
}

.node-order {
  width: 100%;
}

.draggable-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  margin-bottom: 6px;
  cursor: grab;
  background: var(--el-fill-color-light);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
}

.row-number {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color);
  border-radius: 50%;
}

.ghost {
  opacity: 0.55;
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
  padding: 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
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
  .client-row {
    display: block;
  }

  .page-header .el-button,
  .batch-actions,
  .client-actions {
    margin-top: 12px;
  }
}
</style>
