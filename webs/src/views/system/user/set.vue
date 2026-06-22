<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { Check, CopyDocument, Refresh, User } from "@element-plus/icons-vue";
import { useUserStore } from "@/store";
import { updateUserPassword } from "@/api/user";
import { checkUpdate, type UpdateInfo } from "@/api/system/update";

const userStore = useUserStore();
const userinfo = ref<any>();
const saving = ref(false);
const checking = ref(false);
const updateInfo = ref<UpdateInfo>();

const form = reactive({
  username: "",
  password: "",
});

const updateStateType = computed(() => {
  if (!updateInfo.value) return "info";
  return updateInfo.value.hasUpdate ? "warning" : "success";
});

onMounted(async () => {
  userinfo.value = await userStore.getUserInfo();
  form.username = userinfo.value?.username ?? "";
  await loadUpdateInfo();
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

async function loadUpdateInfo() {
  checking.value = true;
  try {
    const { data } = await checkUpdate();
    updateInfo.value = data;
  } finally {
    checking.value = false;
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
</script>

<template>
  <div class="settings-page">
    <section class="settings-hero">
      <div>
        <span>系统设置</span>
        <h1>账号与版本维护</h1>
        <p>这里可以修改管理员登录信息，也可以检查 Docker 镜像是否有新版本。</p>
      </div>
      <el-button :icon="Refresh" @click="loadUpdateInfo" :loading="checking">检查更新</el-button>
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
        </div>

        <div class="command-box" v-if="updateInfo?.updateCommand">
          <code>{{ updateInfo.updateCommand }}</code>
          <el-button :icon="CopyDocument" @click="copyCommand">复制命令</el-button>
        </div>

        <div class="update-actions">
          <el-button :icon="Refresh" :loading="checking" @click="loadUpdateInfo">重新检查</el-button>
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

.settings-form {
  max-width: 420px;
}

.version-list {
  display: grid;
  gap: 12px;
  margin: 16px 0;
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
