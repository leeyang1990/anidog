<template>
  <div class="min-h-screen flex items-center justify-center bg-background ac-grass-pattern p-4 relative overflow-hidden">
    <LocaleSwitcher class="absolute right-4 top-4 z-10" />
    <div class="ac-grove-decoration absolute -left-12 top-12 w-32 h-32 text-ac-grass-light opacity-50 animate-bounce-soft" aria-hidden="true">
      <svg viewBox="0 0 24 24" fill="currentColor" class="w-full h-full"><path d="M12 2 C 5 5, 4 14, 12 22 C 20 14, 19 5, 12 2 Z" /></svg>
    </div>

    <AcCard padding="lg" rounded="3xl" shadow="lg" class="w-full max-w-sm bg-card border-2 border-ac-sand">
      <div class="text-center mb-7">
        <div class="inline-flex items-center justify-center size-16 rounded-3xl bg-ac-sun/30 border-2 border-ac-sun shadow-md mb-4">
          <img src="@/assets/logo.svg" alt="AniDog" class="size-12" />
        </div>
        <h1 class="text-3xl font-bold tracking-tight text-foreground">{{ t('auth.registerTitle') }}</h1>
        <p class="text-sm text-muted-foreground mt-1.5">{{ t('auth.registerSubtitle') }}</p>
      </div>

      <form @submit.prevent="handleSubmit" class="space-y-3.5">
        <div class="space-y-1.5">
          <label class="text-sm font-bold text-foreground">{{ t('auth.username') }}</label>
          <AcInput v-model="formValue.username" :placeholder="t('auth.usernamePlaceholder')" required size="lg" autocomplete="username">
            <template #prefix><PersonOutline class="size-4" /></template>
          </AcInput>
        </div>

        <div class="space-y-1.5">
          <label class="text-sm font-bold text-foreground">{{ t('auth.email') }}</label>
          <AcInput v-model="formValue.email" type="email" :placeholder="t('auth.emailPlaceholder')" required size="lg" autocomplete="email">
            <template #prefix><MailOutline class="size-4" /></template>
          </AcInput>
        </div>

        <div class="space-y-1.5">
          <label class="text-sm font-bold text-foreground">{{ t('auth.password') }}</label>
          <AcInput v-model="formValue.password" type="password" :placeholder="t('auth.passwordCreatePlaceholder')" required size="lg" autocomplete="new-password">
            <template #prefix><LockClosedOutline class="size-4" /></template>
          </AcInput>
        </div>

        <div class="space-y-1.5">
          <label class="text-sm font-bold text-foreground">{{ t('auth.confirmPassword') }}</label>
          <AcInput v-model="formValue.confirmPassword" type="password" :placeholder="t('auth.confirmPasswordPlaceholder')" required size="lg" autocomplete="new-password">
            <template #prefix><ShieldCheckmarkOutline class="size-4" /></template>
          </AcInput>
        </div>

        <AcButton type="submit" variant="sun" size="lg" block :loading="loading" class="!mt-5">
          {{ loading ? t('auth.registering') : t('auth.createAccount') }}
        </AcButton>
      </form>

      <p class="text-center text-sm text-muted-foreground mt-6">
        {{ t('auth.hasAccount') }}
        <router-link to="/auth/login" class="text-ac-grass-dark font-bold hover:underline">{{ t('auth.loginNow') }}</router-link>
      </p>
    </AcCard>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '../../composables/useToast'
import { post } from '@/utils/api'
import {
  PersonOutline, LockClosedOutline, MailOutline, ShieldCheckmarkOutline,
} from '@vicons/ionicons5'
import { AcCard, AcInput, AcButton } from '../../components/ac'
import LocaleSwitcher from '../../components/Common/LocaleSwitcher.vue'
import { useI18n } from 'vue-i18n'

const router = useRouter()
const toast = useToast()
const { t } = useI18n({ useScope: 'global' })

const loading = ref(false)

const formValue = reactive({
  username: '',
  email: '',
  password: '',
  confirmPassword: '',
})

const handleSubmit = async () => {
  if (formValue.password !== formValue.confirmPassword) {
    toast.error(t('auth.passwordMismatch'))
    return
  }
  loading.value = true
  try {
    await post('/auth/register', {
      username: formValue.username,
      email: formValue.email,
      password: formValue.password,
    })
    toast.success(t('auth.registerSuccess'))
    router.push('/auth/login')
  } catch (error) {
    toast.error(error.message || t('auth.registerFailed'))
  } finally {
    loading.value = false
  }
}
</script>
