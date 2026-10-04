<template>
  <div class="logo-container">
    <transition enter-active-class="animate__animated animate__fadeInLeft">
      <router-link v-if="collapse" class="wh-full flex-center" to="/">
        <span class="brand-mark">S<span>↗</span></span>
      </router-link>

      <router-link v-else class="wh-full flex-center" to="/">
        <span class="brand-mark">S<span>↗</span></span>
        <span class="logo-title"
          >{{ defaultSettings.title }}<small>SUBSCRIPTION STUDIO</small></span
        >
      </router-link>
    </transition>
  </div>
</template>

<script lang="ts" setup>
import defaultSettings from "@/settings";
import { useSettingsStore } from "@/store";

const settingsStore = useSettingsStore();

defineProps({
  collapse: {
    type: Boolean,
    required: true,
  },
});
</script>

<style lang="scss" scoped>
.logo-container {
  width: 100%;
  height: $navbar-height;
  padding: 12px 20px;
  background: transparent;

  a {
    height: 38px;
    justify-content: flex-start;
    border: none;
    border-radius: 8px;
    background: transparent;
  }

  .logo-image {
    width: 26px;
    height: 26px;
  }

  .logo-title {
    flex-shrink: 0; /* 防止容器在空间不足时缩小 */
    margin-left: 10px;
    color: #f1f7f4;
    font-size: 19px;
    font-weight: 600;
    letter-spacing: 0;
  }
  .logo-title small {
    display: block;
    margin-top: 1px;
    color: #91b4a8;
    font-size: 8px;
    font-weight: 400;
    letter-spacing: 1.2px;
  }
  .brand-mark {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 1px;
    flex: 0 0 34px;
    height: 34px;
    border-radius: 10px;
    color: #113b2f;
    background: #b9eccf;
    font-size: 22px;
    font-weight: 750;
  }
  .brand-mark span {
    align-self: flex-start;
    padding-top: 2px;
    font-size: 14px;
  }
}

.hideSidebar {
  .logo-container {
    padding-right: 7px;
    padding-left: 7px;

    .logo-image {
      width: 24px;
      height: 24px;
    }
  }
}

.layout-top,
.layout-mix {
  .logo-container {
    width: $sidebar-width;
  }

  &.hideSidebar {
    .logo-container {
      width: $sidebar-width-collapsed;
    }
  }
}
</style>
