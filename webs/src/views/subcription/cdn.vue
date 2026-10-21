<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from "vue";
import { Plus, Refresh, VideoPlay, VideoPause, CopyDocument, Download, Check } from "@element-plus/icons-vue";
import { getCDN, saveCDN, runCDN, cancelCDN, importCDN, type CDNProfile, type CDNResult } from "@/api/subcription/cdn";
import { getNodes, GetGroup } from "@/api/subcription/node";
import { getSubs } from "@/api/subcription/subs";

type SavedNode = { ID: number; Name: string; Link: string; LinkOverride?: string };
const fresh = (): CDNProfile => ({ id: 0, name: "我的 CDN 优选", templateLink: "", ips: "", count: 10,
  url: "https://cf.xiu2.xyz/url", maxDelay: 500, maxLoss: 0,
  minSpeed: 0.1, duration: 5, groupName: "CDN 优选", status: "idle", message: "", results: [] });
const form = ref<CDNProfile>(fresh());
const profiles = ref<CDNProfile[]>([]);
const nodes = ref<SavedNode[]>([]);
const groups = ref<string[]>([]);
const subs = ref<{ ID: number; Name: string }[]>([]);
const selectedNode = ref<number>();
const subscriptionID = ref(0);
const engineReady = ref(false);
const loading = ref(true);
const busy = ref(false);
const current = computed(() => profiles.value.find(p => p.id === form.value.id));
const running = computed(() => profiles.value.find(p => p.status === "running"));
const results = computed(() => current.value?.results || []);
const selected = ref<CDNResult[]>([]);
const table = ref<any>();
const preview = ref("");
const previewOpen = ref(false);
const advanced = ref<string[]>([]);
const statusNames: Record<string, string> = { idle: "待开始", running: "优选中", completed: "已完成", failed: "未完成", cancelled: "已停止", interrupted: "已中断" };
const ipCount = computed(() => new Set(form.value.ips.split(/[\s,，]+/).filter(Boolean)).size);
let timer: ReturnType<typeof setTimeout> | undefined;
let mounted = true;
let resultKey = "";

async function load() {
  const response: any = await getCDN();
  profiles.value = response.data.profiles || [];
  engineReady.value = response.data.engineReady;
  const key = `${current.value?.id}:${current.value?.finishedAt || ""}:${results.value.length}`;
  if (key !== resultKey) {
    resultKey = key;
    selected.value = [];
    await nextTick();
    table.value?.clearSelection();
    results.value.forEach(r => table.value?.toggleRowSelection(r, true));
  }
}
async function poll() {
  if (!mounted) return;
  try { await load(); } catch { /* Request layer displays the connection error. */ }
  if (mounted) timer = setTimeout(poll, running.value ? 2000 : 6000);
}
function chooseProfile(id: number) {
  const p = profiles.value.find(p => p.id === id);
  if (p) form.value = { ...p };
  selectedNode.value = undefined;
  resultKey = "";
  void load();
}
function newProfile() { form.value = fresh(); selectedNode.value = undefined; selected.value = []; }
function useNode(id: number) {
  const n = nodes.value.find(n => n.ID === id);
  if (n) { form.value.templateLink = n.LinkOverride || n.Link; if (!form.value.id) form.value.name = `${n.Name} · 优选`; }
}
async function readFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  if (file.size > 256 * 1024) { ElMessage.warning("IP 文件不能超过 256 KB"); return; }
  form.value.ips = await file.text();
  input.value = "";
}
async function save(start: boolean) {
  busy.value = true;
  try {
    const response: any = await saveCDN(form.value);
    form.value = response.data;
    if (start) await runCDN(form.value.id);
    await load();
    ElMessage.success(start ? "已开始优选，切换页面后仍会继续运行" : "配置已保存");
  } catch { /* Request layer displays validation errors. */ }
  finally { busy.value = false; }
}
async function stop() {
  if (!running.value) return;
  try { await cancelCDN(running.value.id); ElMessage.info("正在停止测速"); await load(); } catch { /* handled */ }
}
async function saveNodes() {
  if (!current.value || !selected.value.length) return;
  busy.value = true;
  try {
    await importCDN(current.value.id, { ips: selected.value.map(r => r.ip), groupName: form.value.groupName, subscriptionID: subscriptionID.value });
    ElMessage.success(subscriptionID.value ? "节点已保存并追加到订阅" : "节点已保存到节点库，重复 IP 已去重");
  } catch { /* handled */ } finally { busy.value = false; }
}
async function copy() {
  try { await navigator.clipboard.writeText(selected.value.map(r => r.link).join("\n")); ElMessage.success("已复制选中链接"); }
  catch { preview.value = selected.value.map(r => r.link).join("\n"); previewOpen.value = true; }
}
function download() {
  const url = URL.createObjectURL(new Blob([selected.value.map(r => r.link).join("\n")], { type: "text/plain;charset=utf-8" }));
  const a = document.createElement("a"); a.href = url; a.download = "cdn-nodes.txt"; a.click(); URL.revokeObjectURL(url);
}
function showLink(link: string) { preview.value = link; previewOpen.value = true; }
function date(raw?: string) { return raw ? new Date(raw).toLocaleString() : "—"; }
onMounted(async () => {
  try {
    const [, n, g, s]: any[] = await Promise.all([load(), getNodes(), GetGroup(), getSubs()]);
    nodes.value = (n.data || []).filter((n: SavedNode) => /^(vless|vmess|trojan):\/\//.test(n.LinkOverride || n.Link));
    groups.value = g.data || []; subs.value = s.data || [];
    if (profiles.value.length) chooseProfile((running.value || profiles.value[0]).id);
  } catch { /* handled */ } finally { loading.value = false; if (mounted) timer = setTimeout(poll, 2000); }
});
onUnmounted(() => { mounted = false; clearTimeout(timer); });
</script>

<template>
  <div v-loading="loading" class="cdn-page">
    <div class="page-heading">
      <div><span class="page-kicker">资源中心</span><h2>CDN 优选</h2><p>从你的候选 IP 中选出更快的入口，保留原节点配置，一次生成多个链接。</p></div>
      <div class="heading-actions"><el-tag :type="engineReady ? 'success' : 'danger'" effect="plain">{{ engineReady ? '测速引擎就绪' : '测速引擎未安装' }}</el-tag><el-button :icon="Plus" @click="newProfile">新建配置</el-button></div>
    </div>
    <div class="steps-strip"><span><b>01</b> 选择原始节点</span><i>→</i><span><b>02</b> 输入候选 IP</span><i>→</i><span><b>03</b> 测速与筛选</span><i>→</i><span><b>04</b> 保存节点</span></div>
    <el-alert v-if="!engineReady" title="未找到 CloudflareSpeedTest，请在运行设备上配置 CFST_PATH。" type="error" :closable="false" show-icon />
    <div class="cdn-layout">
      <section class="panel input-panel">
        <div class="panel-title"><h3>优选配置</h3><el-select v-if="profiles.length" :model-value="form.id || undefined" placeholder="选择已保存配置" aria-label="选择已保存配置" @change="chooseProfile"><el-option v-for="p in profiles" :key="p.id" :label="p.name" :value="p.id" /></el-select></div>
        <el-form label-position="top" :disabled="current?.status === 'running' || busy">
          <el-form-item label="配置名称"><el-input v-model="form.name" maxlength="120" placeholder="例如：家里宽带 CF 优选" /></el-form-item>
          <el-form-item label="原始 CDN 节点">
            <el-select v-model="selectedNode" filterable clearable placeholder="从已有节点中选择，也可以在下方粘贴" aria-label="从节点库选择模板" @change="useNode"><el-option v-for="n in nodes" :key="n.ID" :label="n.Name" :value="n.ID" /></el-select>
            <el-input v-model="form.templateLink" class="template-input" type="textarea" :rows="3" placeholder="vless://… / vmess://… / trojan://…" aria-label="原始节点链接" />
            <span class="field-help">使用已经能正常连接的 TLS CDN 节点。SNI、Host、路径和认证参数会保留。</span>
          </el-form-item>
          <el-form-item>
            <template #label><div class="field-label"><span>候选 IP <small>{{ ipCount }} 个</small></span><label class="file-label">导入 TXT<input type="file" accept=".txt,text/plain" aria-label="导入 IP 文件" @change="readFile" /></label></div></template>
            <el-input v-model="form.ips" type="textarea" :rows="6" placeholder="每行一个公网 IP，支持 IPv4 和 IPv6；也可使用空格或逗号分隔。" aria-label="候选 IP 列表" />
            <span class="field-help">最多 4096 个，只测试你提供的地址；重复项自动去重。</span>
          </el-form-item>
          <div class="form-row"><el-form-item label="优选数量"><el-input-number v-model="form.count" :min="5" :max="10" controls-position="right" /></el-form-item><el-form-item label="每个 IP 下载测速时长"><el-select v-model="form.duration"><el-option v-for="n in [3,5,10,15,20]" :key="n" :label="`${n} 秒`" :value="n" /></el-select></el-form-item></div>
          <el-collapse v-model="advanced"><el-collapse-item name="advanced" title="高级设置 · 测速地址与筛选条件">
            <el-form-item label="HTTPS 下载测速地址"><el-input v-model="form.url" /><span class="field-help">应为候选 IP 所属 CDN 上可下载的大文件。测速地址不可用会导致下载速度为 0。</span></el-form-item>
            <el-form-item label="最大延迟（ms）"><el-input-number v-model="form.maxDelay" :min="1" :max="9999" /></el-form-item>
            <el-form-item label="最大探测失败率"><el-select v-model="form.maxLoss"><el-option label="0%（不接受失败）" :value="0" /><el-option label="25%" :value="0.25" /><el-option label="50%" :value="0.5" /></el-select></el-form-item>
            <el-form-item label="最低下载速度（MB/s）"><el-input-number v-model="form.minSpeed" :min="0" :max="10000" :step="0.1" :precision="2" /></el-form-item>
          </el-collapse-item></el-collapse>
        </el-form>
        <div class="start-actions"><el-button :icon="Check" :disabled="busy || current?.status === 'running'" @click="save(false)">保存配置</el-button><el-button type="primary" :icon="VideoPlay" :disabled="!engineReady || !!running" :loading="busy" @click="save(true)">开始优选</el-button></div>
        <p class="small-note">最多下载测速 {{ form.count * 3 }} 个候选后排序。测速会占用运行设备的出口带宽。</p>
      </section>
      <div class="output-column">
        <section class="panel status-panel">
          <div class="panel-title"><h3>运行状态</h3><el-tag :type="running ? 'warning' : current?.status === 'failed' ? 'danger' : 'info'">{{ statusNames[current?.status || 'idle'] }}</el-tag></div>
          <div v-if="running" class="running-state"><span class="pulse" /><span>{{ running.name }}：{{ running.message }}</span><el-button :icon="VideoPause" @click="stop">停止</el-button></div>
          <p v-else class="status-message">{{ current?.message || '准备好原始节点和 IP 列表后，即可开始优选。' }}</p>
          <div class="status-meta"><span>开始：{{ date(current?.startedAt) }}</span><span>完成：{{ date(current?.finishedAt) }}</span></div>
          <p class="network-note">测速从运行 sublinkx 的设备发起。电脑测试版反映电脑出口；部署到 NAS 后反映 NAS 出口。请确保测速流量直连。</p>
        </section>
        <section class="panel results-panel">
          <div class="panel-title"><div><h3>优选结果 <span class="count-badge">{{ results.length }}</span></h3><p>按下载速度排序，已通过原节点域名的 TLS 校验。</p></div><el-button :icon="Refresh" @click="load">刷新</el-button></div>
          <div v-if="!results.length" class="empty-results"><div class="empty-symbol">↗</div><strong>{{ running ? '正在寻找更好的 CDN 入口' : '优选结果将在这里显示' }}</strong><p>{{ running ? '后台会继续运行，你可以切换到其他页面。' : '仅保留实际达标的 IP，数量不足时不会用失败结果补齐。' }}</p></div>
          <el-table v-else ref="table" :data="results" row-key="ip" @selection-change="selected = $event" class="results-table">
            <el-table-column type="selection" :reserve-selection="true" width="42" /><el-table-column prop="ip" label="IP 地址" min-width="155" show-overflow-tooltip />
            <el-table-column label="延迟" width="90"><template #default="{ row }">{{ row.delay.toFixed(0) }} ms</template></el-table-column>
            <el-table-column label="下载" width="112"><template #default="{ row }"><strong class="speed">{{ row.speed.toFixed(2) }}</strong> MB/s</template></el-table-column>
            <el-table-column label="失败率" width="78"><template #default="{ row }">{{ (row.loss * 100).toFixed(0) }}%</template></el-table-column>
            <el-table-column prop="colo" label="机房" width="70" /><el-table-column label="链接" width="70"><template #default="{ row }"><el-button link type="primary" @click="showLink(row.link)">预览</el-button></template></el-table-column>
          </el-table>
          <div v-if="results.length" class="result-actions"><span>已选 {{ selected.length }} 个</span><el-button :icon="CopyDocument" :disabled="!selected.length" @click="copy">复制链接</el-button><el-button :icon="Download" :disabled="!selected.length" @click="download">导出 TXT</el-button></div>
          <div class="save-area">
            <h4>保存到 sublinkx</h4><div class="form-row"><el-form-item label="节点分组" label-position="top"><el-select v-model="form.groupName" filterable allow-create default-first-option aria-label="保存分组"><el-option label="CDN 优选" value="CDN 优选" /><el-option v-for="g in groups.filter(g => g !== 'CDN 优选')" :key="g" :label="g" :value="g" /></el-select></el-form-item><el-form-item label="追加到订阅（可选）" label-position="top"><el-select v-model="subscriptionID" aria-label="追加到订阅"><el-option label="仅保存到节点库" :value="0" /><el-option v-for="s in subs" :key="s.ID" :label="s.Name" :value="s.ID" /></el-select></el-form-item></div>
            <el-button type="primary" :icon="Check" :disabled="!selected.length || !!running" :loading="busy" @click="saveNodes">保存选中节点</el-button><p class="small-note">原节点保留；相同配置的相同 IP 会更新现有记录。不会移除订阅中的其他节点。</p>
          </div>
          <p class="verification-note">这里验证的是 CDN 速度与 TLS 连通性。完整代理连接还需在你的客户端中测试。</p>
        </section>
      </div>
    </div>
    <el-dialog v-model="previewOpen" title="生成的节点链接" width="min(720px, 94vw)"><el-input :model-value="preview" type="textarea" :rows="7" readonly aria-label="生成的节点链接" /><template #footer><el-button @click="previewOpen = false">关闭</el-button></template></el-dialog>
  </div>
</template>

<style scoped lang="scss">
.cdn-page { padding: 24px; color: var(--sx-text); }
.page-heading, .heading-actions, .panel-title, .field-label, .result-actions, .running-state { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.page-heading { margin-bottom: 20px; flex-wrap: wrap; }
.page-heading h2 { margin: 6px 0 8px; font-size: 26px; font-weight: 650; }
.page-heading p, .panel-title p { margin: 0; color: var(--sx-muted); font-size: 13px; line-height: 1.7; }
.steps-strip { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 16px 24px; margin-bottom: 20px; background: var(--sx-surface); border: 1px solid var(--sx-border); border-radius: 10px; font-size: 13px; }
.steps-strip b { color: var(--el-color-primary); margin-right: 9px; font-size: 12px; }
.steps-strip i { color: #94a3b8; font-style: normal; }
.cdn-layout { display: grid; grid-template-columns: minmax(330px, 0.85fr) minmax(480px, 1.5fr); align-items: start; gap: 20px; }
.panel { min-width: 0; background: var(--sx-surface); border: 1px solid var(--sx-border); border-radius: 10px; padding: 22px; }
.panel-title { margin-bottom: 20px; }
.panel-title h3 { font-size: 16px; font-weight: 600; margin: 0 0 4px; }
.panel-title > .el-select { width: 180px; }
.output-column { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; min-width: 0; }
.field-label { width: 100%; }
.field-label small { font-weight: normal; color: var(--sx-muted); margin-left: 6px; }
.file-label { color: var(--el-color-primary); cursor: pointer; font-size: 12px; position: relative; }
.file-label input { position: absolute; inset: 0; opacity: 0; width: 100%; cursor: pointer; }
.field-help, .small-note { display: block; color: var(--sx-muted); font-size: 12px; line-height: 1.8; margin: 7px 0 0; }
.template-input { margin-top: 10px; }
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.start-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 20px; }
.start-actions .el-button + .el-button { margin: 0; }
.status-message { font-size: 14px; line-height: 1.8; margin-bottom: 16px; }
.status-meta { display: flex; flex-wrap: wrap; gap: 8px 24px; color: var(--sx-muted); font-size: 12px; }
.network-note { background: var(--sx-accent-soft); border-radius: 7px; font-size: 12px; line-height: 1.8; padding: 12px 14px; margin: 16px 0 0; color: var(--sx-muted); }
.running-state { justify-content: flex-start; font-size: 13px; line-height: 1.7; margin-bottom: 16px; }
.running-state .el-button { margin-left: auto; }
.pulse { width: 9px; height: 9px; flex-shrink: 0; background: #f59e0b; border-radius: 50%; }
.count-badge { margin-left: 8px; background: var(--sx-accent-soft); color: var(--el-color-primary); border-radius: 5px; padding: 2px 8px; font-size: 12px; }
.empty-results { text-align: center; padding: 40px 10px; }
.empty-symbol { margin: 0 auto 16px; width: 46px; height: 46px; border-radius: 12px; background: var(--sx-accent-soft); color: var(--el-color-primary); font-size: 28px; line-height: 46px; }
.empty-results strong { font-size: 14px; font-weight: 500; }
.empty-results p { color: var(--sx-muted); font-size: 12px; line-height: 1.8; }
.speed { color: var(--el-color-primary); }
.result-actions { justify-content: flex-end; margin: 16px 0; flex-wrap: wrap; }
.result-actions > span { margin-right: auto; color: var(--sx-muted); font-size: 12px; }
.save-area { border-top: 1px solid var(--sx-border); margin-top: 12px; padding-top: 18px; }
.save-area h4 { margin: 0 0 16px; font-size: 14px; font-weight: 600; }
.verification-note { border-top: 1px solid var(--sx-border); padding-top: 14px; margin: 18px 0 0; color: var(--sx-muted); font-size: 12px; line-height: 1.8; }
:deep(.el-select), :deep(.el-input-number) { width: 100%; }
:deep(.el-form-item__label) { width: 100%; font-size: 13px; }
:deep(.el-textarea__inner) { font-family: ui-monospace, monospace; font-size: 12px; line-height: 1.75; }
:deep(.el-collapse) { border: 0; }
:deep(.el-collapse-item__header) { background: transparent; font-size: 12px; }
:deep(.el-collapse-item__wrap) { background: transparent; }
:deep(.el-table) { --el-table-header-bg-color: var(--sx-accent-soft); }
@media (max-width: 1200px) { .cdn-layout { grid-template-columns: 1fr; } }
@media (max-width: 640px) { .cdn-page { padding: 14px; } .panel { padding: 16px; } .steps-strip { flex-wrap: wrap; padding: 14px; gap: 12px; } .steps-strip i { display: none; } .steps-strip span { width: calc(50% - 8px); } .form-row { grid-template-columns: 1fr; gap: 0; } .panel-title { flex-wrap: wrap; } .page-heading h2 { font-size: 23px; } .heading-actions { width: 100%; } }
</style>
