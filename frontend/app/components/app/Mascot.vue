<!-- corporate mascot: the «ik» mark with hand-drawn limbs, lines follow the theme ink -->
<script setup lang="ts">
withDefaults(defineProps<{
  // idea: lightbulb · fragile: sign in hands · lost: tilted, scribbles around
  // rest: dozing · paid: holds up a check · bell: rings a bell · jump: hops in place (waiting)
  // wave: login · chart: overview · calendar · coin: periods · loop: recurring · badge: users · search: log · gear: settings
  pose?: 'idea' | 'fragile' | 'lost' | 'rest' | 'paid' | 'bell' | 'jump'
    | 'wave' | 'chart' | 'calendar' | 'coin' | 'loop' | 'badge' | 'search' | 'gear'
  size?: number
}>(), { pose: 'idea', size: 160 })
</script>

<template>
  <svg
    class="mascot"
    :class="pose"
    :width="size"
    :height="size * 1.2"
    viewBox="0 0 200 240"
    fill="none"
    role="img"
    aria-label="Талисман Invoice Keeper"
  >
    <g class="lines">
      <template v-if="pose === 'idea'">
        <path d="M100 18C88 18 82 28 86 38c3 6 6 8 6 12h16c0-4 3-6 6-12 4-10-2-20-14-20Z" />
        <path d="M93 55h14M94 59h12M96 63h8M96 43l3-8 3 8 3-8" />
        <path d="M62 102C42 92 34 60 40 26" />
        <path d="M140 108c12 2 20 12 14 20-5 6-10-1-4-6" />
        <path d="M82 150c-2 22-10 36-6 58m-8 0c4 6 14 4 12-2" />
        <path d="M118 150c2 22 8 40 14 56m4 0c6-2 4-10-2-8" />
      </template>

      <template v-else-if="pose === 'fragile'">
        <g transform="rotate(-10 100 120)">
          <rect x="62" y="6" width="76" height="38" rx="2" />
          <path d="M72 14h8l-4 8-4-8Zm4 8v8m-4 0h8" />
          <text x="88" y="30" class="sign">ХРУПКО!</text>
          <path d="M66 100C48 84 54 58 68 44" />
          <path d="M134 100c16-16 10-42-4-56" />
          <path d="M72 148c-22 12-42 4-60 16m-4-6c-6 4 0 12 6 8" />
          <path d="M122 150c6 20 30 28 52 36m4 0c6 2 10-6 4-8" />
        </g>
      </template>

      <template v-else-if="pose === 'wave'">
        <path d="M138 104C158 96 164 74 156 56" />
        <path d="M168 50l6-5M172 62h8M166 40l2-8" />
        <path d="M62 120c-14 10-18 26-12 40m-6 0c4 6 14 4 12-2" />
        <path d="M82 160c-2 20-8 34-4 50m-8 0c4 6 14 4 12-2" />
        <path d="M118 160c2 20 8 34 12 48m4 0c6-2 4-10-2-8" />
      </template>

      <template v-else-if="pose === 'chart'">
        <path d="M148 84v-6" />
        <path d="M150 30v52h42" />
        <rect x="156" y="62" width="8" height="20" />
        <rect x="168" y="48" width="8" height="34" />
        <rect x="180" y="34" width="8" height="48" />
        <path d="M138 108C146 104 148 98 148 88" />
        <path d="M62 120c-14 10-18 26-12 40m-6 0c4 6 14 4 12-2" />
        <path d="M82 160c-2 20-8 34-4 50m-8 0c4 6 14 4 12-2" />
        <path d="M118 160c2 20 8 34 12 48m4 0c6-2 4-10-2-8" />
      </template>

      <template v-else-if="pose === 'calendar'">
        <rect x="68" y="14" width="64" height="50" rx="3" />
        <path d="M68 28h64M84 8v10M116 8v10M78 38h8M96 38h8M114 38h8M78 50h8M96 50h8" />
        <path d="M64 102C46 86 54 64 70 54" />
        <path d="M136 102c16-16 8-38-8-48" />
        <path d="M82 160c-2 20-8 34-4 50m-8 0c4 6 14 4 12-2" />
        <path d="M118 160c2 20 8 34 12 48m4 0c6-2 4-10-2-8" />
      </template>

      <template v-else-if="pose === 'coin'">
        <circle cx="156" cy="58" r="18" />
        <text x="148" y="66" class="zzz" font-size="22">₽</text>
        <path d="M138 108C152 100 158 90 158 78" />
        <path d="M178 34l6-6M184 50h8M130 36l-6-6" />
        <path d="M62 120c-14 10-18 26-12 40m-6 0c4 6 14 4 12-2" />
        <path d="M82 160c-2 20-8 34-4 50m-8 0c4 6 14 4 12-2" />
        <path d="M118 160c2 20 8 34 12 48m4 0c6-2 4-10-2-8" />
      </template>

      <template v-else-if="pose === 'loop'">
        <path d="M170 56a22 22 0 1 1-8-17" />
        <path d="M160 30l2 10 10-2" />
        <path d="M138 108C150 100 152 86 150 74" />
        <path d="M62 120c-14 10-18 26-12 40m-6 0c4 6 14 4 12-2" />
        <path d="M82 160c-2 20-8 34-4 50m-8 0c4 6 14 4 12-2" />
        <path d="M118 160c2 20 8 34 12 48m4 0c6-2 4-10-2-8" />
      </template>

      <template v-else-if="pose === 'badge'">
        <rect x="138" y="30" width="54" height="36" rx="3" />
        <circle cx="154" cy="43" r="5" />
        <path d="M146 58c2-6 12-6 16 0M168 42h16M168 50h12" />
        <path d="M138 108C152 98 158 84 160 66" />
        <path d="M62 120c-14 10-18 26-12 40m-6 0c4 6 14 4 12-2" />
        <path d="M82 160c-2 20-8 34-4 50m-8 0c4 6 14 4 12-2" />
        <path d="M118 160c2 20 8 34 12 48m4 0c6-2 4-10-2-8" />
      </template>

      <template v-else-if="pose === 'search'">
        <circle cx="152" cy="54" r="17" />
        <path d="M140 66L126 84" />
        <path d="M144 48c2-4 6-6 10-5" />
        <path d="M138 110C128 106 124 96 126 84" />
        <path d="M62 120c-14 10-18 26-12 40m-6 0c4 6 14 4 12-2" />
        <path d="M82 160c-2 20-8 34-4 50m-8 0c4 6 14 4 12-2" />
        <path d="M118 160c2 20 8 34 12 48m4 0c6-2 4-10-2-8" />
      </template>

      <template v-else-if="pose === 'gear'">
        <circle cx="156" cy="52" r="12" />
        <circle cx="156" cy="52" r="4" />
        <path d="M156 34v6M156 64v6M138 52h6M168 52h6M143 39l4 4M165 61l4 4M169 39l-4 4M147 61l-4 4" />
        <path d="M138 108C152 100 156 88 156 72" />
        <path d="M62 120c-14 10-18 26-12 40m-6 0c4 6 14 4 12-2" />
        <path d="M82 160c-2 20-8 34-4 50m-8 0c4 6 14 4 12-2" />
        <path d="M118 160c2 20 8 34 12 48m4 0c6-2 4-10-2-8" />
      </template>

      <template v-else-if="pose === 'jump'">
        <path d="M62 104C46 90 44 70 50 56" />
        <path d="M138 104c16-14 18-34 12-48" />
        <path d="M84 160c-8 14-18 20-30 22m-4-6c-4 6 4 10 8 6" />
        <path d="M116 160c8 14 18 20 30 22m4-6c4 6-4 10-8 6" />
      </template>

      <template v-else-if="pose === 'rest'">
        <path d="M62 118C44 126 40 148 46 166m-6 0c4 6 14 4 12-2" />
        <path d="M138 118c18 8 22 30 16 48m4 0c-4 6-14 4-12-2" />
        <path d="M84 160c-2 18 6 30 0 46m-6 0c4 5 14 3 12-2" />
        <path d="M116 160c2 18-6 30 0 46m6 0c-4 5-14 3-12-2" />
        <text x="132" y="62" class="zzz" font-size="20">Z</text>
        <text x="152" y="42" class="zzz" font-size="15">z</text>
        <text x="166" y="26" class="zzz" font-size="11">z</text>
      </template>

      <template v-else-if="pose === 'paid'">
        <circle cx="152" cy="48" r="20" />
        <path d="M141 49l7 8 14-16" />
        <path d="M138 106C158 98 160 80 154 70" />
        <path d="M62 120c-14 10-18 26-12 40m-6 0c4 6 14 4 12-2" />
        <path d="M82 160c-2 20-8 34-4 50m-8 0c4 6 14 4 12-2" />
        <path d="M118 160c2 20 8 34 12 48m4 0c6-2 4-10-2-8" />
        <path d="M176 22l4-8M186 40l8-2M120 28l-4-8" />
      </template>

      <template v-else-if="pose === 'bell'">
        <path d="M28 72c0-4 2-6 2-14 0-8 6-14 12-14s12 6 12 14c0 8 2 10 2 14Z" />
        <path d="M38 77c0 4 8 4 8 0M42 44v-4" />
        <path d="M16 52l-8-4M18 40l-6-8M66 40l6-8M68 54l8-4" />
        <path d="M62 106C44 98 38 88 40 78" />
        <path d="M140 108c12 2 20 12 14 20-5 6-10-1-4-6" />
        <path d="M82 150c-2 22-10 36-6 58m-8 0c4 6 14 4 12-2" />
        <path d="M118 150c2 22 8 40 14 56m4 0c6-2 4-10-2-8" />
      </template>

      <template v-else>
        <g transform="rotate(14 100 120)">
          <path d="M60 110C40 112 24 104 8 118" />
          <path d="M140 110c18-4 34 2 50-8m4-2c4-6-4-12-8-6" />
          <path d="M74 156c-6 24 4 44-10 70" />
          <path d="M126 156c8 22-2 44 12 70" />
        </g>
        <path d="M30 40c10-14 26-10 20 2s-22 4-12-4" />
        <path d="M150 28c14-6 24 6 12 14-10 6-18-6-8-10" />
        <path d="M178 54c-8-6-4-18 6-16 8 2 6 14-2 14-6 0-6-8 0-8" />
        <path d="M160 200h26M160 214h22" />
      </template>
    </g>

    <g
      class="body"
      :transform="pose === 'fragile' ? 'rotate(-10 100 120)' : pose === 'lost' ? 'rotate(14 100 120)' : undefined"
    >
      <rect x="60" y="80" width="80" height="80" rx="20" fill="#2B3EF0" />
      <g transform="translate(60 80) scale(4.444)" fill="#fff">
        <path d="M9.42579 13.1169V15.2627C9.42579 15.8046 8.98649 16.2439 8.44458 16.2439C7.90268 16.2439 7.46338 15.8046 7.46338 15.2627V4.49341C7.46338 3.95151 7.90268 3.51221 8.44458 3.51221C8.98649 3.51221 9.42579 3.95151 9.42579 4.49341V10.5284C9.42579 10.5806 9.46816 10.623 9.52043 10.623C9.54641 10.623 9.57125 10.6123 9.58912 10.5935L13.0434 6.94789C13.1963 6.78653 13.4089 6.69513 13.6311 6.69513C14.3423 6.69513 14.708 7.54637 14.2183 8.06218L12.6887 9.67362C12.0946 10.2995 12.0404 11.2633 12.5608 11.9517L14.6961 14.777C15.1518 15.3799 14.7217 16.2439 13.9661 16.2439C13.6755 16.2439 13.4023 16.106 13.2298 15.8722L10.8504 12.6481C10.5636 12.2595 9.9982 12.2172 9.6568 12.5589C9.50888 12.7069 9.42579 12.9077 9.42579 13.1169Z" />
        <path d="M5.70733 4.39025V15.3659C5.70733 15.8508 5.31421 16.2439 4.82928 16.2439C4.34435 16.2439 3.95123 15.8508 3.95123 15.3659V4.39026C3.95123 3.90532 4.34435 3.51221 4.82928 3.51221C5.31421 3.51221 5.70733 3.90532 5.70733 4.39025Z" />
        <path d="M4.84076 1.09082C5.4056 1.09082 5.86322 1.54942 5.86322 2.11426C5.86304 2.67894 5.40548 3.13672 4.84076 3.13672C4.27607 3.13667 3.81848 2.67891 3.8183 2.11426C3.8183 1.54945 4.27596 1.09087 4.84076 1.09082ZM4.84076 1.40527C4.6236 1.40537 4.4472 1.58164 4.4472 1.79883C4.44724 1.92088 4.50446 2.02846 4.59174 2.10059L4.4472 2.82129H5.23431L5.08978 2.10059C5.17718 2.02846 5.23428 1.92098 5.23431 1.79883C5.23431 1.58158 5.058 1.40527 4.84076 1.40527Z" />
      </g>
    </g>
  </svg>
</template>

<style scoped>
.mascot {
  flex-shrink: 0;
  overflow: visible;
}

.lines {
  stroke: var(--ink);
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-linejoin: round;
}

/* the line stays as thin when the mascot is small */
.lines :is(path, circle, rect) {
  vector-effect: non-scaling-stroke;
}

.zzz {
  fill: var(--ink);
  stroke: none;
  font-weight: 600;
}

.sign {
  fill: var(--ink);
  stroke: none;
  font-size: 9px;
  font-weight: 600;
  letter-spacing: 0.04em;
}

/* a slow float, off for reduced motion */
.body {
  transform-box: fill-box;
  transform-origin: center;
  animation: mascot-float 4s ease-in-out infinite;
}

@keyframes mascot-float {
  50% {
    translate: 0 -3px;
  }
}

/* waiting: hops in place, body and limbs together */
.jump .body,
.jump .lines {
  animation: mascot-jump 0.7s cubic-bezier(0.33, 0, 0.67, 1) infinite;
}

@keyframes mascot-jump {
  50% {
    translate: 0 -26px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .body,
  .jump .body,
  .jump .lines {
    animation: none;
  }
}
</style>
