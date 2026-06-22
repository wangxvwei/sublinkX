<template>
  <div class="flex">
    <template v-if="!isMobile">
      <!--全屏 -->
      <div class="setting-item" @click="toggle">
        <svg-icon
          :icon-class="isFullscreen ? 'fullscreen-exit' : 'fullscreen'"
        />
      </div>

      <!-- 布局大小 -->
      <el-tooltip
        :content="$t('sizeSelect.tooltip')"
        effect="dark"
        placement="bottom"
      >
        <size-select class="setting-item" />
      </el-tooltip>

      <!-- 语言选择 -->
      <lang-select class="setting-item" />
    </template>

    <!-- 用户头像 -->
    <router-link class="update-shortcut" to="/system/user/set">
      <el-icon><Refresh /></el-icon>
      <span>更新</span>
    </router-link>

    <el-dropdown class="setting-item" trigger="click">
      <div class="user-trigger">
        <img
          :src="userStore.user.avatar + '?imageView2/1/w/80/h/80'"
          class="rounded-full mr-10px w24px w24px"
        />
        <span>{{ userStore.user.username }}</span>
      </div>
      <template #dropdown>
        <el-dropdown-menu>
            <router-link to="/system/user/set">
            <el-dropdown-item>系统设置 / 版本更新</el-dropdown-item>
            </router-link>
          <el-dropdown-item divided @click="logout">
            {{ $t("navbar.logout") }}
          </el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>

    <!-- 设置 -->
    <template v-if="defaultSettings.showSettings">
      <div class="setting-item" @click="settingStore.settingsVisible = true">
        <svg-icon icon-class="setting" />
      </div>
    </template>
  </div>
</template>
<script setup lang="ts">
import {
  useAppStore,
  useTagsViewStore,
  useUserStore,
  useSettingsStore,
} from "@/store";
import defaultSettings from "@/settings";
import { DeviceEnum } from "@/enums/DeviceEnum";
import { Refresh } from "@element-plus/icons-vue";

const appStore = useAppStore();
const tagsViewStore = useTagsViewStore();
const userStore = useUserStore();
const settingStore = useSettingsStore();

const route = useRoute();
const router = useRouter();

const isMobile = computed(() => appStore.device === DeviceEnum.MOBILE);

const { isFullscreen, toggle } = useFullscreen();

/**
 * 注销
 */
function logout() {
  ElMessageBox.confirm("确定注销并退出系统吗？", "提示", {
    confirmButtonText: "确定",
    cancelButtonText: "取消",
    type: "warning",
    lockScroll: false,
  }).then(() => {
    userStore
      .logout()
      .then(() => {
        tagsViewStore.delAllViews();
      })
      .then(() => {
        router.push(`/login?redirect=${route.fullPath}`);
      });
  });
}
</script>
<style lang="scss" scoped>
.setting-item {
  display: inline-block;
  min-width: 40px;
  height: $navbar-height;
  line-height: $navbar-height;
  color: #111827;
  text-align: center;
  cursor: pointer;

  &:hover {
    background: #eff6ff;
  }
}

.setting-item :deep(.svg-icon),
.setting-item :deep(.el-icon),
.setting-item span {
  color: #111827;
}

.user-trigger {
  display: flex;
  align-items: center;
  height: 100%;
  padding: 0 12px;
  color: #111827;
  font-weight: 650;
}

.update-shortcut {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 12px;
  margin: 9px 6px 0 2px;
  color: #1d4ed8;
  font-size: 14px;
  font-weight: 700;
  line-height: 34px;
  text-decoration: none;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  background: #eff6ff;
  transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease;
}

.update-shortcut:hover {
  color: #ffffff;
  border-color: #2563eb;
  background: #2563eb;
}

.dark .setting-item:hover {
  background: rgb(255 255 255 / 20%);
}

.dark .setting-item,
.dark .setting-item :deep(.svg-icon),
.dark .setting-item :deep(.el-icon),
.dark .setting-item span,
.dark .user-trigger {
  color: #e5e7eb;
}
</style>
