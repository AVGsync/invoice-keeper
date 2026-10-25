<script setup lang="ts">
const { open } = useNav()
const route = useRoute()

watch(() => route.fullPath, () => (open.value = false))

// the page behind the open drawer doesn't scroll
watch(open, (value) => {
  document.documentElement.style.overflow = value ? 'hidden' : ''
})

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') open.value = false
}
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKey)
  document.documentElement.style.overflow = ''
})
</script>

<template>
  <div class="panel">
    <!-- phones only: the menu lives in a drawer, this bar opens it -->
    <header class="topbar">
      <button type="button" class="burger" :aria-label="open ? 'Закрыть меню' : 'Открыть меню'" :aria-expanded="open" @click="open = !open">
        <svg viewBox="0 0 20 20" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" aria-hidden="true">
          <path d="M3 5.5h14M3 10h14M3 14.5h14" />
        </svg>
      </button>
      <AppLogo />
    </header>

    <AppSidebar />
    <div class="scrim" :class="{ show: open }" aria-hidden="true" @click="open = false" />

    <main class="main">
      <slot />
    </main>
    <AppToast />
  </div>
</template>

<style scoped>
.panel {
  display: flex;
  min-height: 100vh;
}

.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 0 32px 0 31px;
}

.topbar,
.scrim {
  display: none;
}

@media (max-width: 1240px) {
  .panel {
    flex-direction: column;
  }

  .topbar {
    position: sticky;
    top: 0;
    z-index: 40;
    display: flex;
    align-items: center;
    gap: 6px;
    height: calc(52px + env(safe-area-inset-top, 0px));
    padding: env(safe-area-inset-top, 0px) 12px 0 6px;
    background: var(--bg);
    border-bottom: 1px solid var(--line);
  }

  .burger {
    display: grid;
    place-items: center;
    width: 44px;
    height: 44px;
    background: none;
    border: 0;
    border-radius: 9999px;
    color: var(--ink);
  }

  .burger:active {
    background: var(--subtle);
  }

  .scrim {
    position: fixed;
    inset: 0;
    z-index: 55;
    display: block;
    background: rgb(0 0 0 / 0.55);
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.25s ease;
  }

  .scrim.show {
    opacity: 1;
    pointer-events: auto;
  }

  /* on a tablet the column stays phone-width in the middle instead of stretching */
  .main {
    width: 100%;
    max-width: 820px;
    margin: 0 auto;
    padding: 0 16px calc(24px + env(safe-area-inset-bottom, 0px));
  }
}
</style>
