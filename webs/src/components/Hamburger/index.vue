<template>
  <button
    type="button"
    class="sidebar-trigger px-[15px] flex items-center justify-center color-[var(--el-text-color-regular)]"
    :aria-label="isActive ? '收起导航' : '展开导航'"
    :aria-expanded="isActive"
    @click="toggleClick"
  >
    <svg-icon
      class="hamburger"
      :class="{ 'is-active': isActive }"
      icon-class="indent-decrease"
    />
  </button>
</template>
<script setup lang="ts">
defineProps({
  isActive: {
    required: true,
    type: Boolean,
    default: false,
  },
});

const emit = defineEmits(["toggleClick"]);

function toggleClick() {
  emit("toggleClick");
}
</script>

<style scoped lang="scss">
.sidebar-trigger {
  min-width: 44px;
  min-height: 44px;
  padding: 0 15px;
  background: transparent;
  cursor: pointer;
}
.hamburger {
  vertical-align: middle;
  cursor: pointer;
  transform: scaleX(-1);
}

.hamburger.is-active {
  transform: scaleX(1);
}
</style>
