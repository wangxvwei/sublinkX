<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from "vue";
import { Plus, Refresh, VideoPlay, VideoPause, CopyDocument, Download, Check } from "@element-plus/icons-vue";
import { getCDN, saveCDN, runCDN, cancelCDN, importCDN, setCDNPinnedIP, type CDNProfile, type CDNResult } from "@/api/subcription/cdn";
import { getNodes } from "@/api/subcription/node";
import { getSubs } from "@/api/subcription/subs";

type SavedNode = { ID: number; Name: string; Link: string; LinkOverride?: string };
const fresh = (): CDNProfile => ({ id: 0, name: "", templateLink: "", ips: "", count: 10,
  candidateMode: "recommend", recommendationMode: "single", sourceIP: "", carrier: "",
  url: "https://cf.xiu2.xyz/url", maxDelay: 500, maxLoss: 0,
  minSpeed: 0.1, duration: 5, groupName: "", status: "idle", message: "", results: [] });
const form = ref<CDNProfile>(fresh());
const profiles = ref<CDNProfile[]>([]);
const nodes = ref<SavedNode[]>([]);
const subs = ref<{ ID: number; Name: string }[]>([]);
const selectedNode = ref<number>();
const subscriptionID = ref(0);
const engineReady = ref(false);
const loading = ref(true);
const busy = ref(false);
const current = computed(() => profiles.value.find(p => p.id === form.value.id));
const running = computed(() => profiles.value.find(p => p.status === "running"));
const results = computed(() => current.value?.results || []);
const network = computed(() => current.value?.network);
const pinnedIPs = computed(() => current.value?.pinnedIPs || []);
const recommending = computed(() => form.value.candidateMode === "recommend");
const recommendedResults = computed(() => current.value ? current.value.candidateMode === "recommend" : recommending.value);
const recommendationNames = { single: "单源推荐（原版麒麟）", mixed: "多源混合推荐" };
// Older results predate the mode field; their source statuses identify the run.
const resultMode = computed(() => network.value?.recommendationMode || (network.value?.source ? ((network.value.sources?.length || 0) > 1 ? "mixed" : "single") : (current.value?.recommendationMode || "single")));
const carrierNames: Record<string, string> = { CT: "电信", CU: "联通", CM: "移动" };
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
  const key = `${current.value?.id}:${current.value?.updatedAt || ""}:${results.value.length}`;
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
  if (p) form.value = { ...p, candidateMode: p.candidateMode || "manual", recommendationMode: p.recommendationMode || "single" };
  selectedNode.value = undefined;
  resultKey = "";
  void load();
}
function newProfile(copy = false) {
  form.value = copy ? { ...form.value, id: 0, name: "", groupName: "", status: "idle", message: "", results: [], pinnedIPs: [], network: undefined, startedAt: undefined, finishedAt: undefined, updatedAt: undefined } : fresh();
  if (!copy) selectedNode.value = undefined;
  subscriptionID.value = 0;
  selected.value = [];
  resultKey = "";
  table.value?.clearSelection();
}
async function renameProfile() {
  if (!current.value) return;
  try {
    const { value } = await ElMessageBox.prompt("只修改当前配置及其节点、分组名称。新增地点请使用“复制为新配置”。", "重命名配置", {
      inputValue: current.value.name, inputValidator: (value: string) => !!value?.trim() || "请填写配置名称",
      confirmButtonText: "确认重命名", cancelButtonText: "取消"
    });
    busy.value = true;
    const response: any = await saveCDN({ ...current.value, name: value }, true);
    form.value.name = response.data.name;
    await load();
    ElMessage.success("配置、分组和节点已重命名");
  } catch { /* Cancel or validation error; request layer handles errors. */ }
  finally { busy.value = false; }
}
function useNode(id: number) {
  const n = nodes.value.find(n => n.ID === id);
  if (n) form.value.templateLink = n.LinkOverride || n.Link;
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
    ElMessage.success(start ? "任务已开始，切换页面后仍会继续运行" : "配置已保存");
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
    await importCDN(current.value.id, { ips: selected.value.map(r => r.ip), subscriptionID: subscriptionID.value });
    ElMessage.success(`“${current.value.name}”已更新为本次选中的 ${selected.value.length} 个节点`);
  } catch { /* handled */ } finally { busy.value = false; }
}
async function keepIP(ip: string, keep: boolean) {
  if (!current.value) return;
  const id = current.value.id;
  const chosen = new Set(selected.value.map(r => r.ip));
  busy.value = true;
  try {
    await setCDNPinnedIP(id, ip, keep);
    await load();
    if (current.value?.id === id) {
      table.value?.clearSelection();
      results.value.filter(r => chosen.has(r.ip)).forEach(r => table.value?.toggleRowSelection(r, true));
    }
    ElMessage.success(keep ? "后续推荐会优先保留这个 IP" : "已取消保留");
  } catch { /* request layer handles errors */ }
  finally { busy.value = false; }
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
    const [, n, s]: any[] = await Promise.all([load(), getNodes(), getSubs()]);
    nodes.value = (n.data || []).filter((n: SavedNode) => /^(vless|vmess|trojan):\/\//.test(n.LinkOverride || n.Link));
    subs.value = s.data || [];
    if (profiles.value.length) chooseProfile((running.value || profiles.value[0]).id);
  } catch { /* handled */ } finally { loading.value = false; if (mounted) timer = setTimeout(poll, 2000); }
});
onUnmounted(() => { mounted = false; clearTimeout(timer); });
</script>

<template>
  <div v-loading="loading" class="cdn-page">
    <div class="page-heading">
      <div><span class="page-kicker">资源中心</span><h2>CDN 优选</h2><p>填写目标地区公网 IP，识别地区与运营商，生成可直接导入的 CDN 推荐节点。</p></div>
      <div class="heading-actions"><el-tag v-if="recommending" type="warning" effect="plain">异地推荐 · 非目标地区实测</el-tag><el-tag v-else :type="engineReady ? 'success' : 'danger'" effect="plain">{{ engineReady ? '测速引擎就绪' : '测速引擎未安装' }}</el-tag><el-button :icon="Plus" :disabled="busy" @click="newProfile()">新建配置</el-button></div>
    </div>
    <div class="steps-strip"><span><b>01</b> 选择原始节点</span><i>→</i><span><b>02</b> {{ recommending ? '填写目标公网 IP' : '填写 CDN 地址' }}</span><i>→</i><span><b>03</b> {{ recommending ? '匹配公开线路推荐' : '本机测速' }}</span><i>→</i><span><b>04</b> 保存节点</span></div>
    <el-alert v-if="!engineReady && !recommending" title="未找到 CloudflareSpeedTest，请在运行设备上配置 CFST_PATH。" type="error" :closable="false" show-icon />
    <div class="cdn-layout">
      <section class="panel input-panel">
        <div class="panel-title"><h3>优选配置</h3><el-select v-if="profiles.length" :model-value="form.id || undefined" :disabled="busy" placeholder="选择已保存配置" aria-label="选择已保存配置" @change="chooseProfile"><el-option v-for="p in profiles" :key="p.id" :label="p.name" :value="p.id" /></el-select></div>
        <el-form label-position="top" :disabled="current?.status === 'running' || busy">
          <el-form-item :label="form.id ? '正在编辑的配置' : '新配置名称'" required><el-input v-model="form.name" :readonly="!!form.id" maxlength="120" placeholder="请填写名称，例如：西大优选" aria-label="配置名称" />
            <div v-if="form.id" class="profile-actions"><el-button link type="primary" @click="newProfile(true)">复制为新配置</el-button><el-button link @click="renameProfile">重命名</el-button></div>
            <span class="field-help">{{ form.id ? '增加其他地点时，点击“复制为新配置”后填写新名称，当前配置会保留。' : '保存后新增一个独立配置。分组使用此名称，节点按“名称 · 01、02…”命名。' }}</span></el-form-item>
          <el-form-item label="原始 CDN 节点">
            <el-select v-model="selectedNode" filterable clearable placeholder="从已有节点中选择，也可以在下方粘贴" aria-label="从节点库选择模板" @change="useNode"><el-option v-for="n in nodes" :key="n.ID" :label="n.Name" :value="n.ID" /></el-select>
            <el-input v-model="form.templateLink" class="template-input" type="textarea" :rows="3" placeholder="vless://… / vmess://… / trojan://…" aria-label="原始节点链接" />
            <span class="field-help">使用已经能正常连接的 TLS CDN 节点。SNI、Host、路径和认证参数会保留。</span>
          </el-form-item>
          <el-form-item label="优选方式">
            <el-select v-model="form.candidateMode" aria-label="优选方式">
              <el-option label="异地线路推荐（无需当地设备）" value="recommend" />
              <el-option label="本机测速（手动提供 CDN 地址）" value="manual" />
            </el-select>
          </el-form-item>
          <template v-if="recommending">
            <el-form-item label="推荐来源模式">
              <el-select v-model="form.recommendationMode" aria-label="推荐来源模式"><el-option v-for="(label, mode) in recommendationNames" :key="mode" :label="label" :value="mode" /></el-select>
              <span class="field-help">{{ form.recommendationMode === 'mixed' ? '混合麒麟、WeTest、CFYes，综合公开排名、新鲜度与上榜记录。候选更多，不保证通过率更高。' : '沿用原版麒麟列表顺序；除你保留的好用 IP 外，只使用麒麟来源，不混入其他来源。' }}</span>
              <span class="field-help">每个地点独立保存。切换后重新生成才生效，好用 IP 保留；保存结果时才更新节点库。</span>
            </el-form-item>
            <el-form-item label="目标地区公网 IPv4">
              <el-input v-model="form.sourceIP" maxlength="45" placeholder="例如填写 B 省用户的公网 IP，与 NAS 所在地无关" aria-label="目标地区公网 IPv4" />
              <span class="field-help">你在 A 省也可以填写 B 省的公网 IP。用于识别目标地区和运营商，不会拿这个 IP 当 CDN 地址测速。</span>
            </el-form-item>
            <el-form-item label="目标运营商">
              <el-select v-model="form.carrier" placeholder="根据公网 IP 自动识别" aria-label="目标运营商"><el-option label="根据公网 IP 自动识别" value="" /><el-option v-for="(name, code) in carrierNames" :key="code" :label="name" :value="code" /></el-select>
              <span class="field-help">识别不准确时可手动指定。所选来源按运营商提供通用推荐，不代表该省实测。</span>
            </el-form-item>
            <el-form-item v-if="current" label="保留好用的 IP">
              <div v-if="pinnedIPs.length" class="pinned-list"><el-tag v-for="ip in pinnedIPs" :key="ip" :closable="!busy && !running" @close="keepIP(ip, false)">{{ ip }}</el-tag></div>
              <span v-else class="field-help">在推荐结果中点击“保留好用”，后续生成时优先保留。</span>
              <span class="field-help">单源与多源切换时保留标记。修改原始节点、目标公网 IP、运营商或改为本机测速后会清除标记。保存节点以本次勾选为准。</span>
            </el-form-item>
          </template>
          <el-form-item v-else>
            <template #label><div class="field-label"><span>候选 IP <small>{{ ipCount }} 个</small></span><label class="file-label">导入 TXT<input type="file" accept=".txt,text/plain" aria-label="导入 IP 文件" @change="readFile" /></label></div></template>
            <el-input v-model="form.ips" type="textarea" :rows="6" placeholder="每行一个公网 IP，支持 IPv4 和 IPv6；也可使用空格或逗号分隔。" aria-label="候选 IP 列表" />
            <span class="field-help">这里填 CDN 服务器地址，不是公网出口 IP。最多 4096 个；想按异地公网 IP 推荐，请切换上方模式。</span>
          </el-form-item>
          <div class="form-row"><el-form-item :label="recommending ? '推荐数量' : '优选数量'"><el-input-number v-model="form.count" :min="5" :max="10" controls-position="right" /></el-form-item><el-form-item v-if="!recommending" label="每个 IP 下载测速时长"><el-select v-model="form.duration"><el-option v-for="n in [3,5,10,15,20]" :key="n" :label="`${n} 秒`" :value="n" /></el-select></el-form-item></div>
          <el-collapse v-if="!recommending" v-model="advanced"><el-collapse-item name="advanced" title="高级设置 · 测速地址与筛选条件">
            <el-form-item label="HTTPS 下载测速地址"><el-input v-model="form.url" /><span class="field-help">应为候选 IP 所属 CDN 上可下载的大文件。测速地址不可用会导致下载速度为 0。</span></el-form-item>
            <el-form-item label="最大延迟（ms）"><el-input-number v-model="form.maxDelay" :min="1" :max="9999" /></el-form-item>
            <el-form-item label="最大探测失败率"><el-select v-model="form.maxLoss"><el-option label="0%（不接受失败）" :value="0" /><el-option label="25%" :value="0.25" /><el-option label="50%" :value="0.5" /></el-select></el-form-item>
            <el-form-item label="最低下载速度（MB/s）"><el-input-number v-model="form.minSpeed" :min="0" :max="10000" :step="0.1" :precision="2" /></el-form-item>
          </el-collapse-item></el-collapse>
        </el-form>
        <div class="start-actions"><el-button :icon="Check" :disabled="busy || current?.status === 'running'" @click="save(false)">{{ form.id ? '保存当前配置' : '创建新配置' }}</el-button><el-button type="primary" :icon="VideoPlay" :disabled="(!recommending && !engineReady) || !!running" :loading="busy" @click="save(true)">{{ recommending ? '生成线路推荐' : '开始本机测速' }}</el-button></div>
        <p v-if="recommending" class="small-note">仅支持 Cloudflare。按公开数据源的运营商排序推荐，不使用 A 省的测速结果推断 B 省速度。</p>
        <p v-else class="small-note">最多下载测速 {{ form.count * 3 }} 个候选后排序。测速会占用运行设备的出口带宽。</p>
      </section>
      <div class="output-column">
        <section class="panel status-panel">
          <div class="panel-title"><h3>运行状态</h3><el-tag :type="running ? 'warning' : current?.status === 'failed' ? 'danger' : 'info'">{{ statusNames[current?.status || 'idle'] }}</el-tag></div>
          <div v-if="running" class="running-state"><span class="pulse" /><span>{{ running.name }}：{{ running.message }}</span><el-button :icon="VideoPause" @click="stop">停止</el-button></div>
          <p v-else class="status-message">{{ current?.message || '选择原节点，填写目标公网 IP，即可生成线路推荐。' }}</p>
          <div class="status-meta"><span>开始：{{ date(current?.startedAt) }}</span><span>完成：{{ date(current?.finishedAt) }}</span></div>
          <div v-if="network?.location || network?.warning" class="network-details">
            <p v-if="recommendedResults">本次结果模式：<strong>{{ recommendationNames[resultMode] }}</strong></p>
            <p v-if="current?.sourceIP">目标公网 IP：<strong>{{ current.sourceIP }}</strong><br />{{ network?.location }}</p>
            <p v-if="network?.carrier">目标地区：{{ network.province || '未识别' }} · {{ carrierNames[network.carrier] }}<br />匹配范围：同运营商通用推荐（非省级实测）</p>
            <div v-if="network?.sources?.length" class="feed-statuses">
              <p>公开候选 {{ network.candidateCount || 0 }} 个 · 已积累 {{ network.historyWindows || 0 }} 个历史时段</p>
              <div v-for="feed in network.sources" :key="feed.id" class="feed-status"><a :href="feed.url" target="_blank" rel="noopener noreferrer">{{ feed.name }}</a><el-tag :type="feed.error ? 'warning' : feed.count ? 'success' : 'info'" size="small">{{ feed.cached ? `缓存 · ${feed.count} 个地址` : feed.error ? '暂不可用' : `${feed.count} 个匹配地址` }}</el-tag><span>{{ feed.cached ? `请求失败，使用有效期内缓存；上游更新：${date(feed.updatedAt)}` : feed.error || `上游更新：${date(feed.updatedAt)}` }}</span></div>
              <span class="field-help">{{ resultMode === 'mixed' ? '三个原始来源均经 BestCF 公开镜像读取。同源镜像不重复加分；来源失效时使用其余有效数据。' : '麒麟来源经 BestCF 公开镜像读取。来源失效时只使用麒麟的有效缓存和你保留的 IP，不自动切换其他来源。' }}</span>
              <span class="field-help">历史是最近 7 天公开列表的上榜记录，不是节点连接成功率。按上游时间的 30 分钟窗口去重；单源模式不按历史重新排序。</span>
            </div>
            <p v-else-if="network?.source">数据源：<a :href="network.source" target="_blank" rel="noopener noreferrer">麒麟优选 · BestCF 公开镜像</a><br />数据源更新时间：{{ date(network.sourceUpdatedAt) }}</p>
            <el-alert v-if="network?.warning" :title="network.warning" type="warning" :closable="false" show-icon />
            <span class="field-help">归属地：太平洋 IP 查询；运营商不足时参考 ipapi.is 的 ASN 信息。</span>
          </div>
          <p class="network-note">{{ recommending ? '公网 IP 只用于识别目标线路。推荐不代表目标地区已经测过，也不保证可用或最快；生成后由使用者在目标网络验证。' : '测速从运行 sublinkx 的设备发起，结果只代表这台设备的实际出口。请确保测速流量直连。' }}</p>
        </section>
        <section class="panel results-panel">
          <div class="panel-title"><div><h3>{{ recommendedResults ? '线路推荐' : '优选结果' }} <span class="count-badge">{{ results.length }}</span></h3><p>{{ recommendedResults ? (resultMode === 'mixed' ? '优先保留好用 IP，再综合多个来源、新鲜度与上榜记录；非目标地区实测。' : '优先保留好用 IP，其余按麒麟原始顺序选择；非目标地区实测。') : '按下载速度排序，已通过原节点域名的 TLS 校验。' }}</p></div><el-button :icon="Refresh" @click="load">刷新</el-button></div>
          <div v-if="!results.length" class="empty-results"><div class="empty-symbol">↗</div><strong>{{ running ? '任务正在后台运行' : '结果将在这里显示' }}</strong><p>{{ running ? '你可以切换到其他页面。' : recommendedResults ? '按目标运营商匹配公开地址；不足时显示实际数量，不虚构结果。' : '仅保留实际达标的 IP，数量不足时不会用失败结果补齐。' }}</p></div>
          <el-table v-else ref="table" :data="results" row-key="ip" @selection-change="selected = $event" class="results-table">
            <el-table-column type="selection" :reserve-selection="true" width="42" /><el-table-column prop="name" label="节点名称" min-width="160" show-overflow-tooltip /><el-table-column prop="ip" label="IP 地址" min-width="155" show-overflow-tooltip />
            <el-table-column v-if="recommendedResults" label="推荐依据" min-width="230"><template #default="{ row }"><el-tag v-if="row.pinned" type="success" size="small">用户保留</el-tag><div>{{ row.basis || `${carrierNames[network?.carrier || '']}通用 · 非目标省实测` }}</div><small v-if="row.sources?.length">{{ row.sources.join('、') }} · {{ date(row.sourceUpdatedAt) }}</small></template></el-table-column>
            <el-table-column v-if="recommendedResults" label="好用 IP" width="100"><template #default="{ row }"><el-button link :type="row.pinned ? 'success' : 'primary'" :disabled="busy || !!running" @click="keepIP(row.ip, !row.pinned)">{{ row.pinned ? '取消保留' : '保留好用' }}</el-button></template></el-table-column>
            <template v-else>
              <el-table-column label="延迟" width="90"><template #default="{ row }">{{ row.delay.toFixed(0) }} ms</template></el-table-column>
              <el-table-column label="下载" width="112"><template #default="{ row }"><strong class="speed">{{ row.speed.toFixed(2) }}</strong> MB/s</template></el-table-column>
              <el-table-column label="失败率" width="78"><template #default="{ row }">{{ (row.loss * 100).toFixed(0) }}%</template></el-table-column>
              <el-table-column prop="colo" label="机房" width="70" />
            </template>
            <el-table-column label="链接" width="70"><template #default="{ row }"><el-button link type="primary" @click="showLink(row.link)">预览</el-button></template></el-table-column>
          </el-table>
          <div v-if="results.length" class="result-actions"><span>已选 {{ selected.length }} 个</span><el-button :icon="CopyDocument" :disabled="!selected.length" @click="copy">复制链接</el-button><el-button :icon="Download" :disabled="!selected.length" @click="download">导出 TXT</el-button></div>
          <div class="save-area">
            <h4>保存到 sublinkx</h4><div class="form-row"><el-form-item label="节点分组" label-position="top"><el-input :model-value="current?.name || form.name" readonly placeholder="请先填写并保存配置名称" aria-label="节点分组" /></el-form-item><el-form-item label="追加到订阅（可选）" label-position="top"><el-select v-model="subscriptionID" aria-label="追加到订阅"><el-option label="仅保存到节点库" :value="0" /><el-option v-for="s in subs" :key="s.ID" :label="s.Name" :value="s.ID" /></el-select></el-form-item></div>
            <el-button type="primary" :icon="Check" :disabled="!selected.length || !!running" :loading="busy" @click="saveNodes">更新为选中的 {{ selected.length }} 个节点</el-button><p class="small-note">当前配置的节点将替换为本次所选数量，旧批次不再累积；已有订阅同步更新。其他配置和原始节点保留。</p>
          </div>
          <p class="verification-note">{{ recommendedResults ? '推荐地址已校验属于 Cloudflare 官方网段，保留原节点端口和配置；速度、TLS 和完整代理连接均需在目标网络测试。' : '这里验证的是 CDN 速度与 TLS 连通性。完整代理连接还需在你的客户端中测试。' }}</p>
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
.profile-actions { display: flex; flex-wrap: wrap; gap: 14px; margin-top: 8px; }
.pinned-list { display: flex; gap: 8px; flex-wrap: wrap; }
.feed-statuses { margin: 12px 0; }
.feed-status { display: flex; flex-wrap: wrap; gap: 6px 10px; margin: 8px 0; align-items: center; }
.feed-status > span:last-child { flex-basis: 100%; color: var(--sx-muted); }
.template-input { margin-top: 10px; }
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.start-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 20px; }
.start-actions .el-button + .el-button { margin: 0; }
.status-message { font-size: 14px; line-height: 1.8; margin-bottom: 16px; }
.status-meta { display: flex; flex-wrap: wrap; gap: 8px 24px; color: var(--sx-muted); font-size: 12px; }
.network-note { background: var(--sx-accent-soft); border-radius: 7px; font-size: 12px; line-height: 1.8; padding: 12px 14px; margin: 16px 0 0; color: var(--sx-muted); }
.network-details { margin-top: 16px; font-size: 13px; line-height: 1.8; overflow-wrap: anywhere; }
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
