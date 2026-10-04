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
          WORKSPACE / 工作空间
        </div>
        <SidebarMenu :menu-list="permissionStore.routes" base-path="" />
      </el-scrollbar>
      <div
        v-if="layout === 'left' && appStore.sidebar.opened"
        class="sidebar-guide"
      >
        <span>从资源到订阅</span>
        <p>接入来源 → 整理节点 → 分发订阅</p>
        <router-link to="/system/update"
          >sublinkX · v{{ defaultSettings.version }}
          <span>检查更新 ↗</span></router-link
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
    height: calc(100vh - $navbar-height - 130px);
  }
}
.nav-caption {
  padding: 18px 24px 10px;
  color: #76978f;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 1.2px;
}
.sidebar-guide {
  position: absolute;
  bottom: 16px;
  left: 16px;
  right: 16px;
  padding: 16px;
  border: 1px solid #31584e;
  border-radius: 14px;
  color: #d6e6e0;
  background: #173f35;
  font-size: 12px;
}
.sidebar-guide p {
  margin: 7px 0 18px;
  color: #9eb8b0;
  font-size: 11px;
}
.sidebar-guide a {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #9eb8b0;
  font-size: 10px;
}
.sidebar-guide a span {
  color: #ade2cb;
}
.hideSidebar .el-scrollbar {
  height: calc(100vh - $navbar-height);
}
.layout-top .el-scrollbar,
.layout-mix .el-scrollbar {
  height: $navbar-height;
}
</style>
