<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from "vue";
import { Check, CopyDocument, Refresh, User } from "@element-plus/icons-vue";
import { useUserStore } from "@/store";
import { updateUserPassword } from "@/api/user";
import {
  applyUpdate,
  checkUpdate,
  getUpdateStatus,
  type UpdateInfo,
  type UpdateStatus,
} from "@/api/system/update";

const userStore = useUserStore();
const userinfo = ref<any>();
const saving = ref(false);
const checking = ref(false);
const updating = ref(false);
const updateInfo = ref<UpdateInfo>();
const updateStatus = ref<UpdateStatus>();
let updateStatusTimer: number | undefined;
let updateStatusFailures = 0;

const form = reactive({
  username: "",
  password: "",
});

const updateStateType = computed(() => {
  if (!updateInfo.value) return "info";
  return updateInfo.value.hasUpdate ? "warning" : "success";
});

const updateStatusVisible = computed(
  () => updateStatus.value && updateStatus.value.status !== "idle"
);

const updateStatusClass = computed(() => `is-${updateStatus.value?.status || "idle"}`);

const updateStatusMeta = computed(() => {
  if (!updateStatus.value) return "";
  const items = [
    updateStatus.value.targetImage ? `目标镜像：${updateStatus.value.targetImage}` : "",
    updateStatus.value.finishedAt ? `完成时间：${formatDate(updateStatus.value.finishedAt)}` : "",
  ].filter(Boolean);
  return items.join(" · ");
});

onMounted(async () => {
  userinfo.value = await userStore.getUserInfo();
  form.username = userinfo.value?.username ?? "";
  await loadUpdateInfo();
  await loadUpdateStatus(false);
});

onUnmounted(() => {
  stopUpdateStatusPolling();
});

async function resetPassword() {
  if (!form.username.trim() || !form.password.trim()) {
    ElMessage.warning("账号和新密码不能为空");
    return;
  }
  if (form.password.length < 6) {
    ElMessage.warning("密码不能少于 6 位");
    return;
  }

  await ElMessageBox.confirm("确定修改管理员账号和密码吗？修改后需要重新登录。", "修改账号", {
    confirmButtonText: "确认修改",
    cancelButtonText: "取消",
    type: "warning",
  });

  saving.value = true;
  try {
    await updateUserPassword({
      username: form.username.trim(),
      password: form.password.trim(),
    });
    ElMessage.success("账号信息已更新，请重新登录");
    window.location.reload();
  } finally {
    saving.value = false;
  }
}

async function loadUpdateInfo(showToast = false) {
  checking.value = true;
  try {
    const { data } = await checkUpdate();
    updateInfo.value = data;
    if (showToast) {
      if (!data?.latestVersion) {
        ElMessage.warning(data?.message || "暂时无法获取最新版本");
      } else if (data.hasUpdate) {
        ElMessage.warning(`发现新版本：${data.latestVersion}`);
      } else {
        ElMessage.success(`当前已是最新版本：${data.currentVersion || data.latestVersion}`);
      }
    }
  } finally {
    checking.value = false;
  }
}

async function loadUpdateStatus(showFinalMessage = true) {
  try {
    const { data } = await getUpdateStatus();
    updateStatus.value = data;
    if (data?.status === "running") {
      beginUpdateStatusPolling();
      return;
    }
    if (showFinalMessage) {
      showTerminalUpdateMessage(data);
    }
  } catch (error) {
    console.error(error);
  }
}

async function copyCommand() {
  const command = updateInfo.value?.updateCommand;
  if (!command) return;
  try {
    await navigator.clipboard.writeText(command);
  } catch {
    const textarea = document.createElement("textarea");
    textarea.value = command;
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand("copy");
    document.body.removeChild(textarea);
  }
  ElMessage.success("更新命令已复制");
}

async function startOneClickUpdate() {
  if (!updateInfo.value?.autoUpdate) {
    ElMessage.warning(updateInfo.value?.autoUpdateMessage || "当前容器还没有启用一键更新");
    return;
  }

  await ElMessageBox.confirm(
    "确定开始一键更新吗？系统会拉取 latest 镜像并重建当前容器，页面会短暂无法访问。",
    "一键更新",
    {
      confirmButtonText: "开始更新",
      cancelButtonText: "取消",
      type: "warning",
    }
  );

  updating.value = true;
  try {
    const { data } = await applyUpdate();
    updateStatus.value = {
      status: "running",
      message: data?.message || "更新已开始，容器会短暂重启。",
      targetImage: data?.dockerImage,
    };
    ElMessage.success(data?.message || "更新已开始，请稍后刷新页面");
    beginUpdateStatusPolling();
  } finally {
    updating.value = false;
  }
}

function beginUpdateStatusPolling() {
  stopUpdateStatusPolling();
  updateStatusFailures = 0;
  updateStatusTimer = window.setInterval(async () => {
    try {
      const { data } = await getUpdateStatus();
      updateStatusFailures = 0;
      updateStatus.value = data;
      if (data?.status && data.status !== "running" && data.status !== "idle") {
        stopUpdateStatusPolling();
        showTerminalUpdateMessage(data);
        await loadUpdateInfo(false);
      }
    } catch (error) {
      updateStatusFailures += 1;
      if (updateStatusFailures > 40) {
        stopUpdateStatusPolling();
        ElMessage.warning("更新状态暂时无法获取，请稍后刷新页面查看。");
      }
    }
  }, 3000);
}

function stopUpdateStatusPolling() {
  if (updateStatusTimer) {
    window.clearInterval(updateStatusTimer);
    updateStatusTimer = undefined;
  }
}

function showTerminalUpdateMessage(status?: UpdateStatus) {
  if (!status) return;
  if (status.status === "success") {
    ElMessage.success(status.message || "更新成功，请刷新页面");
  } else if (status.status === "rolled_back") {
    ElMessage.error(status.message || "更新失败，已回滚到上一版本");
  } else if (status.status === "failed") {
    ElMessage.error(status.message || "更新失败，请手动检查容器日志");
  }
}

function formatDate(value?: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function formatDigest(value?: string) {
  if (!value) return "未获取到";
  return value.length > 22 ? `${value.slice(0, 22)}...` : value;
}
</script>

<template>
  <div class="settings-page">
    <section class="settings-hero">
      <div>
        <span>系统设置</span>
        <h1>账号与版本维护</h1>
        <p>这里可以修改管理员登录信息，也可以检查 Docker 镜像是否有新版本。</p>
      </div>
      <el-button :icon="Refresh" @click="loadUpdateInfo(true)" :loading="checking">检查更新</el-button>
    </section>

    <section class="settings-grid">
      <div class="panel">
        <div class="panel-head">
          <div class="panel-icon user">
            <el-icon><User /></el-icon>
          </div>
          <div>
            <h2>管理员账号</h2>
            <p>修改后会重置当前管理员账号和密码。</p>
          </div>
        </div>

        <div v-if="userinfo" class="profile-line">
          <el-avatar :size="48" :src="userinfo.avatar" />
          <div>
            <strong>{{ userinfo.nickname || "管理员" }}</strong>
            <span>{{ userinfo.username }}</span>
          </div>
        </div>

        <el-form label-position="top" class="settings-form">
          <el-form-item label="新账号">
            <el-input v-model="form.username" placeholder="请输入新账号" />
          </el-form-item>
          <el-form-item label="新密码">
            <el-input v-model="form.password" type="password" show-password placeholder="至少 6 位" />
          </el-form-item>
          <el-button type="primary" :loading="saving" @click="resetPassword">保存账号</el-button>
        </el-form>
      </div>

      <div class="panel">
        <div class="panel-head">
          <div class="panel-icon update">
            <el-icon><Check /></el-icon>
          </div>
          <div>
            <h2>版本更新</h2>
            <p>NAS / Docker 部署建议通过拉取新镜像更新。</p>
          </div>
        </div>

        <el-alert
          v-if="updateInfo"
          :title="updateInfo.message"
          :type="updateStateType"
          show-icon
          :closable="false"
        />

        <div class="version-list" v-if="updateInfo">
          <div>
            <span>当前版本</span>
            <strong>{{ updateInfo.currentVersion || "-" }}</strong>
          </div>
          <div>
            <span>最新版本</span>
            <strong>{{ updateInfo.latestVersion || "未获取到" }}</strong>
          </div>
          <div>
            <span>Docker 镜像</span>
            <strong>{{ updateInfo.dockerImage }}</strong>
          </div>
          <div>
            <span>当前镜像 digest</span>
            <strong :title="updateInfo.currentImageDigest || ''">
              {{ formatDigest(updateInfo.currentImageDigest) }}
            </strong>
          </div>
          <div>
            <span>最新镜像 digest</span>
            <strong :title="updateInfo.latestImageDigest || ''">
              {{ formatDigest(updateInfo.latestImageDigest) }}
            </strong>
          </div>
          <div>
            <span>当前容器</span>
            <strong>{{ updateInfo.containerName || "sublinkx" }}</strong>
          </div>
          <div>
            <span>网页一键更新</span>
            <strong :class="updateInfo.autoUpdate ? 'status-ok' : 'status-warn'">
              {{ updateInfo.autoUpdate ? "已启用" : "未启用" }}
            </strong>
          </div>
        </div>

        <el-alert
          v-if="updateInfo?.autoUpdateMessage"
          :title="updateInfo.autoUpdateMessage"
          :type="updateInfo.autoUpdate ? 'success' : 'warning'"
          show-icon
          :closable="false"
          class="update-tip"
        />

        <div v-if="updateStatusVisible" class="update-status-card" :class="updateStatusClass">
          <strong>{{ updateStatus?.message }}</strong>
          <span v-if="updateStatusMeta">{{ updateStatusMeta }}</span>
          <small v-if="updateStatus?.error">{{ updateStatus.error }}</small>
        </div>

        <div class="command-box" v-if="updateInfo?.updateCommand">
          <code>{{ updateInfo.updateCommand }}</code>
          <el-button :icon="CopyDocument" @click="copyCommand">复制命令</el-button>
        </div>

        <div class="update-actions">
          <el-button :icon="Refresh" :loading="checking" @click="loadUpdateInfo(true)">重新检查</el-button>
          <el-button
            type="success"
            :icon="Refresh"
            :loading="updating"
            :disabled="!updateInfo?.autoUpdate"
            @click="startOneClickUpdate"
          >
            一键更新
          </el-button>
          <el-button
            v-if="updateInfo?.releaseUrl"
            type="primary"
            tag="a"
            :href="updateInfo.releaseUrl"
            target="_blank"
          >
            查看发布页
          </el-button>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.settings-page {
  min-height: 100%;
  padding: 20px;
  color: #1f2937;
  background:
    linear-gradient(180deg, rgba(240, 249, 255, 0.95), rgba(248, 250, 252, 0.45) 260px),
    #f6f8fb;
}

.settings-hero,
.panel {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
}

.settings-hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  padding: 22px 24px;
  margin-bottom: 16px;
}

.settings-hero span {
  color: #2563eb;
  font-size: 13px;
  font-weight: 750;
}

.settings-hero h1 {
  margin: 8px 0 6px;
  font-size: 25px;
  font-weight: 760;
}

.settings-hero p,
.panel-head p,
.profile-line span,
.version-list span {
  margin: 0;
  color: #64748b;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.panel {
  padding: 20px;
}

.panel-head {
  display: flex;
  gap: 14px;
  margin-bottom: 18px;
}

.panel-head h2 {
  margin: 0 0 5px;
  font-size: 18px;
  font-weight: 730;
}

.panel-icon {
  display: grid;
  width: 44px;
  height: 44px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 8px;
  font-size: 22px;
}

.panel-icon.user {
  color: #1d4ed8;
  background: #dbeafe;
}

.panel-icon.update {
  color: #047857;
  background: #d1fae5;
}

.profile-line {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  margin-bottom: 16px;
  border-radius: 8px;
  background: #f8fafc;
}

.profile-line strong,
.version-list strong {
  display: block;
  color: #111827;
}

.version-list .status-ok {
  color: #047857;
}

.version-list .status-warn {
  color: #b45309;
}

.settings-form {
  max-width: 420px;
}

.version-list {
  display: grid;
  gap: 12px;
  margin: 16px 0;
}

.update-tip {
  margin-bottom: 14px;
}

.update-status-card {
  display: grid;
  gap: 6px;
  padding: 12px 14px;
  margin-bottom: 14px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  color: #1e3a8a;
  background: #eff6ff;
}

.update-status-card strong {
  font-size: 14px;
}

.update-status-card span,
.update-status-card small {
  color: #64748b;
  line-height: 1.6;
}

.update-status-card small {
  overflow-wrap: anywhere;
}

.update-status-card.is-success {
  border-color: #bbf7d0;
  color: #166534;
  background: #f0fdf4;
}

.update-status-card.is-rolled_back,
.update-status-card.is-failed {
  border-color: #fecdd3;
  color: #be123c;
  background: #fff1f2;
}

.command-box {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #0f172a;
}

.command-box code {
  min-width: 0;
  overflow-wrap: anywhere;
  color: #e2e8f0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 12px;
}

.update-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 16px;
}

@media (max-width: 900px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .settings-page {
    padding: 12px;
  }

  .settings-hero,
  .command-box {
    display: block;
  }

  .settings-hero .el-button,
  .command-box .el-button {
    margin-top: 12px;
  }
}
</style>
