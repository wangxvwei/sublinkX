<template>
  <div class="header-tools">
    <el-tooltip
      :content="settingStore.theme === 'dark' ? '切换浅色主题' : '切换深色主题'"
    >
      <el-button
        class="theme-toggle"
        circle
        :icon="settingStore.theme === 'dark' ? Sunny : Moon"
        aria-label="切换主题"
        @click="
          settingStore.changeTheme(
            settingStore.theme === 'dark' ? 'light' : 'dark'
          )
        "
      />
    </el-tooltip>
    <router-link class="update-shortcut" to="/system/update"
      ><el-icon><Refresh /></el-icon><span>版本更新</span></router-link
    >
    <el-dropdown trigger="click">
      <button class="user-trigger" type="button">
        <span class="user-avatar">{{
          (userStore.user.username || "A").slice(0, 1).toUpperCase()
        }}</span
        ><span class="user-name"
          >{{ userStore.user.username }}<small>管理员</small></span
        ><el-icon><ArrowDown /></el-icon>
      </button>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item @click="router.push('/system/account')"
            >账号与安全</el-dropdown-item
          >
          <el-dropdown-item @click="settingStore.settingsVisible = true"
            >外观偏好</el-dropdown-item
          >
          <el-dropdown-item divided @click="logout">退出登录</el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
  </div>
</template>
<script setup lang="ts">
import { useTagsViewStore, useUserStore, useSettingsStore } from "@/store";
import { Refresh, Moon, Sunny, ArrowDown } from "@element-plus/icons-vue";
const tagsViewStore = useTagsViewStore();
const userStore = useUserStore();
const settingStore = useSettingsStore();
const route = useRoute();
const router = useRouter();
function logout() {
  ElMessageBox.confirm("确定退出登录吗？", "退出登录", {
    confirmButtonText: "退出",
    cancelButtonText: "取消",
    type: "warning",
    lockScroll: false,
  })
    .then(async () => {
      await userStore.logout();
      tagsViewStore.delAllViews();
      router.push(`/login?redirect=${route.fullPath}`);
    })
    .catch(() => {});
}
</script>
<style lang="scss" scoped>
.header-tools {
  display: flex;
  align-items: center;
  gap: 14px;
  padding-right: 8px;
}
.theme-toggle {
  border-color: var(--sx-border);
  color: var(--sx-muted);
  background: transparent;
}
.update-shortcut {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--sx-muted);
  font-size: 12px;
}
.user-trigger {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0;
  color: var(--sx-text);
  background: transparent;
  cursor: pointer;
}
.user-avatar {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid var(--sx-border);
  border-radius: 50%;
  color: var(--el-color-primary);
  background: var(--sx-accent-soft);
  font-size: 13px;
  font-weight: 600;
}
.user-name {
  text-align: left;
  font-size: 12px;
}
.user-name small {
  display: block;
  color: var(--sx-muted);
  font-size: 10px;
}
@media (max-width: 760px) {
  .header-tools {
    gap: 9px;
  }
  .user-name,
  .update-shortcut span {
    display: none;
  }
}
</style>
