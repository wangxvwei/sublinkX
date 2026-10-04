<template>
  <div :class="{ 'has-logo': sidebarLogo }">
    <!--混合布局-->
    <div class="flex w-full" v-if="layout == 'mix'">
      <SidebarLogo v-if="sidebarLogo" :collapse="!appStore.sidebar.opened" />
      <SidebarMixTopMenu class="flex-1" />
      <NavbarRight />
    </div>
    <!--左侧布局 || 顶部布局 -->
    <template v-else>
      <SidebarLogo v-if="sidebarLogo" :collapse="!appStore.sidebar.opened" />
      <el-scrollbar>
        <div
          v-if="layout === 'left' && appStore.sidebar.opened"
          class="nav-caption"
        >
          工作空间
        </div>
        <SidebarMenu :menu-list="permissionStore.routes" base-path="" />
      </el-scrollbar>
      <div
        v-if="layout === 'left' && appStore.sidebar.opened"
        class="sidebar-guide"
      >
        <router-link to="/system/update"
          >v{{ defaultSettings.version }} <span>检查更新 →</span></router-link
        >
      </div>
      <NavbarRight v-if="layout === 'top'" />
    </template>
  </div>
</template>

<script setup lang="ts">
import { useSettingsStore, usePermissionStore, useAppStore } from "@/store";
import defaultSettings from "@/settings";

const appStore = useAppStore();
const settingsStore = useSettingsStore();
const permissionStore = usePermissionStore();

const sidebarLogo = computed(() => settingsStore.sidebarLogo);
const layout = computed(() => settingsStore.layout);
</script>

<style lang="scss" scoped>
.has-logo {
  .el-scrollbar {
    height: calc(100vh - $navbar-height - 68px);
  }
}
.nav-caption {
  padding: 20px 24px 12px;
  color: var(--sx-muted);
  font-size: 12px;
  font-weight: 600;
}
.sidebar-guide {
  position: absolute;
  bottom: 16px;
  left: 16px;
  right: 16px;
  padding: 16px 8px 0;
  border-top: 1px solid var(--sx-border);
  color: var(--sx-muted);
  font-size: 12px;
}
.sidebar-guide a {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--sx-muted);
  font-size: 12px;
}
.sidebar-guide a span {
  color: var(--sx-text);
}
.hideSidebar .el-scrollbar {
  height: calc(100vh - $navbar-height);
}
.layout-top .el-scrollbar,
.layout-mix .el-scrollbar {
  height: $navbar-height;
}
</style>
