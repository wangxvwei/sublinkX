<template>
  <div class="login-container">
    <header class="login-header">
      <div class="login-wordmark">
        <span class="login-mark"
          ><el-icon><Connection /></el-icon></span
        >sublinkX
      </div>
      <div class="login-preferences">
        <el-switch
          v-model="isDark"
          inline-prompt
          :active-icon="Moon"
          :inactive-icon="Sunny"
          aria-label="切换深色主题"
          @change="toggleTheme"
        />
        <lang-select class="cursor-pointer" />
      </div>
    </header>
    <main class="login-main">
      <section class="login-card" aria-labelledby="login-title">
        <div class="login-intro">
          <h1 id="login-title">登录工作空间</h1>
          <p>管理你的来源、节点与订阅。</p>
        </div>

        <el-form
          ref="loginFormRef"
          :model="loginData"
          :rules="loginRules"
          class="login-form"
          label-position="top"
          @submit.prevent
        >
          <!-- 用户名 -->
          <el-form-item prop="username" :label="$t('login.username')">
            <el-input
              ref="username"
              v-model="loginData.username"
              :placeholder="$t('login.username')"
              name="username"
              autocomplete="username"
              size="large"
            />
          </el-form-item>

          <!-- 密码 -->
          <el-tooltip
            :visible="isCapslock"
            :content="$t('login.capsLock')"
            placement="right"
          >
            <el-form-item prop="password" :label="$t('login.password')">
              <el-input
                v-model="loginData.password"
                :placeholder="$t('login.password')"
                type="password"
                name="password"
                autocomplete="current-password"
                @keyup="checkCapslock"
                @keyup.enter="handleLogin"
                size="large"
                show-password
              />
            </el-form-item>
          </el-tooltip>

          <!-- 验证码 -->
          <el-form-item prop="captchaCode" :label="$t('login.captchaCode')">
            <div class="captcha-row">
              <el-input
                v-model="loginData.captchaCode"
                auto-complete="off"
                size="large"
                :placeholder="$t('login.captchaCode')"
                @keyup.enter="handleLogin"
              />

              <button
                type="button"
                class="captcha-button"
                title="点击刷新验证码"
                aria-label="刷新验证码"
                @click="getCaptcha"
              >
                <el-image :src="captchaBase64" alt="验证码" fit="contain" />
              </button>
            </div>
          </el-form-item>

          <!-- 登录按钮 -->
          <el-button
            :loading="loading"
            type="primary"
            size="large"
            class="login-submit"
            @click.prevent="handleLogin"
            >{{ $t("login.login") }}
          </el-button>
        </el-form>
        <p class="captcha-help">看不清验证码？点击图片换一张。</p>
      </section>
    </main>
    <footer class="login-footer">
      sublinkX <span>{{ version }}</span>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { useSettingsStore, useUserStore } from "@/store";
import { getCaptchaApi, GetVersion } from "@/api/auth";
import { LoginData } from "@/api/auth/types";
import { Sunny, Moon, Connection } from "@element-plus/icons-vue";
import { LocationQuery, LocationQueryValue, useRoute } from "vue-router";
import router from "@/router";
import { ThemeEnum } from "@/enums/ThemeEnum";
// 获取版本号
const version = ref("");
const fetchVersion = (function () {
  GetVersion()
    .then((res) => {
      console.log("Version fetched:", res.data); // 输出返回内容
      version.value = res.data;
    })
    .catch((error) => {
      console.error("Error fetching version:", error);
    });
})();

// Stores
const userStore = useUserStore();
const settingsStore = useSettingsStore();

// Internationalization
const { t } = useI18n();

// Reactive states
const isDark = ref(settingsStore.theme === ThemeEnum.DARK);
const loading = ref(false); // 按钮loading
const isCapslock = ref(false); // 是否大写锁定
const captchaBase64 = ref(); // 验证码图片Base64字符串
const loginFormRef = ref(ElForm); // 登录表单ref

const loginData = ref<LoginData>({
  username: "",
  password: "",
});

const loginRules = computed(() => {
  return {
    username: [
      {
        required: true,
        trigger: "blur",
        message: t("login.message.username.required"),
      },
    ],
    password: [
      {
        required: true,
        trigger: "blur",
        message: t("login.message.password.required"),
      },
      {
        min: 6,
        message: t("login.message.password.min"),
        trigger: "blur",
      },
    ],
    captchaCode: [
      {
        required: true,
        trigger: "blur",
        message: t("login.message.captchaCode.required"),
      },
    ],
  };
});

/**
 * 获取验证码
 */
function getCaptcha() {
  getCaptchaApi().then(({ data }) => {
    loginData.value.captchaKey = data.captchaKey;
    captchaBase64.value = data.captchaBase64;
  });
}

/**
 * 登录
 */
const route = useRoute();
function handleLogin() {
  loginFormRef.value.validate((valid: boolean) => {
    if (valid) {
      loading.value = true;
      userStore
        .login(loginData.value)
        .then(() => {
          const query: LocationQuery = route.query;
          const redirect = (query.redirect as LocationQueryValue) ?? "/";
          const otherQueryParams = Object.keys(query).reduce(
            (acc: any, cur: string) => {
              if (cur !== "redirect") {
                acc[cur] = query[cur];
              }
              return acc;
            },
            {}
          );

          router.push({ path: redirect, query: otherQueryParams });
        })
        .catch(() => {
          getCaptcha();
        })
        .finally(() => {
          loading.value = false;
        });
    }
  });
}

/**
 * 主题切换
 */

const toggleTheme = () => {
  const newTheme =
    settingsStore.theme === ThemeEnum.DARK ? ThemeEnum.LIGHT : ThemeEnum.DARK;
  settingsStore.changeTheme(newTheme);
};
/**
 * 检查输入大小写
 */
function checkCapslock(event: KeyboardEvent) {
  // 防止浏览器密码自动填充时报错
  if (event instanceof KeyboardEvent) {
    isCapslock.value = event.getModifierState("CapsLock");
  }
}

onMounted(() => {
  getCaptcha();
});
</script>

<style lang="scss" scoped>
.login-container {
  display: grid;
  grid-template-rows: auto 1fr auto;
  min-height: 100vh;
  min-height: 100dvh;
  background: var(--sx-page);
  color: var(--sx-text);
}
.login-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 24px 32px;
}
.login-wordmark {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 20px;
  font-weight: 650;
  letter-spacing: -0.5px;
}
.login-mark {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: var(--el-color-primary);
  color: white;
  font-size: 20px;
}
.login-preferences {
  display: flex;
  align-items: center;
  gap: 18px;
}
.login-main {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 20px 64px;
}
.login-card {
  width: min(420px, 100%);
  padding: 36px;
  border: 1px solid var(--sx-border);
  border-radius: 12px;
  background: var(--sx-surface);
  box-shadow: 0 8px 32px rgb(15 23 42 / 3%);
}
.login-intro {
  margin-bottom: 28px;
}
.login-intro h1 {
  margin: 0 0 8px;
  font-size: 24px;
  font-weight: 600;
  letter-spacing: -0.5px;
}
.login-intro p {
  margin: 0;
  color: var(--sx-muted);
  font-size: 14px;
}
.login-form {
  :deep(.el-form-item) {
    margin-bottom: 22px;
  }
  :deep(.el-form-item__label) {
    color: var(--sx-text);
    font-size: 13px;
    font-weight: 500;
    padding: 0;
    margin-bottom: 8px;
    line-height: 20px;
  }
  :deep(.el-form-item__label::before) {
    display: none;
  }
  :deep(.el-input__wrapper) {
    min-height: 42px;
    border-radius: 6px;
  }
}
.captcha-row {
  display: flex;
  align-items: stretch;
  gap: 10px;
  width: 100%;
}
.captcha-row .el-input {
  min-width: 0;
  flex: 1;
}
.captcha-button {
  display: block;
  padding: 0;
  width: 120px;
  height: 42px;
  flex-shrink: 0;
  overflow: hidden;
  border: 1px solid var(--sx-border);
  border-radius: 6px;
  background: white;
  cursor: pointer;

  .el-image {
    width: 100%;
    height: 100%;
  }
  &:focus-visible {
    outline: 2px solid var(--el-color-primary);
    outline-offset: 2px;
  }
}
.login-submit {
  width: 100%;
  height: 42px;
  margin-top: 2px;
  font-size: 14px;
}
.captcha-help {
  margin: 18px 0 0;
  color: var(--sx-muted);
  font-size: 12px;
  text-align: center;
}
.login-footer {
  padding: 20px;
  color: var(--sx-muted);
  font-size: 12px;
  text-align: center;
  span {
    margin-left: 8px;
  }
}
@media (max-width: 480px) {
  .login-header {
    padding: 20px;
  }
  .login-main {
    padding: 24px 16px 40px;
  }
  .login-card {
    padding: 28px 24px;
  }
  .login-intro h1 {
    font-size: 22px;
  }
}
</style>
