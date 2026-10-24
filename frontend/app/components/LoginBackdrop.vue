<!-- slow rings behind the receipt on the login screen, pure decoration.
     recurring payments go round and round, so: orbits with small dots on them -->
<script setup lang="ts">
// radius, how long one turn takes, which way, and where the dots sit on that ring (degrees)
const rings = [
  { r: 250, turn: 70, reverse: false, dashed: false, dots: [20, 200] },
  { r: 350, turn: 110, reverse: true, dashed: true, dots: [130] },
  { r: 460, turn: 160, reverse: false, dashed: false, dots: [75, 250, 310] },
  { r: 580, turn: 220, reverse: true, dashed: true, dots: [] },
]

// twelve short marks on the second ring, like months on a dial
const ticks = Array.from({ length: 12 }, (_, i) => i * 30)

const at = (r: number, deg: number) => {
  const rad = (deg * Math.PI) / 180
  return { x: 600 + r * Math.sin(rad), y: 600 - r * Math.cos(rad) }
}
</script>

<template>
  <svg class="backdrop" viewBox="0 0 1200 1200" aria-hidden="true">
    <defs>
      <radialGradient id="login-glow">
        <stop offset="0" stop-color="var(--accent)" stop-opacity="0.16" />
        <stop offset="1" stop-color="var(--accent)" stop-opacity="0" />
      </radialGradient>
    </defs>

    <circle class="glow" cx="600" cy="600" r="420" fill="url(#login-glow)" />

    <g
      v-for="(ring, i) in rings"
      :key="ring.r"
      class="ring"
      :class="{ reverse: ring.reverse }"
      :style="{ animationDuration: `${ring.turn}s` }"
    >
      <circle cx="600" cy="600" :r="ring.r" class="line" :class="{ dashed: ring.dashed }" />
      <template v-if="i === 1">
        <line
          v-for="deg in ticks"
          :key="deg"
          class="tick"
          :x1="at(ring.r - 7, deg).x"
          :y1="at(ring.r - 7, deg).y"
          :x2="at(ring.r + 7, deg).x"
          :y2="at(ring.r + 7, deg).y"
        />
      </template>
      <circle v-for="deg in ring.dots" :key="deg" class="dot" :cx="at(ring.r, deg).x" :cy="at(ring.r, deg).y" r="4" />
    </g>

    <!-- the one blue dot, a bit faster than everything else -->
    <g class="ring" :style="{ animationDuration: '46s' }">
      <circle class="accent-halo" :cx="at(250, 110).x" :cy="at(250, 110).y" r="14" />
      <circle class="accent" :cx="at(250, 110).x" :cy="at(250, 110).y" r="5" />
    </g>
  </svg>
</template>

<style scoped>
.backdrop {
  position: absolute;
  /* centred on the receipt */
  top: 506px;
  left: 50%;
  width: 1200px;
  height: 1200px;
  transform: translate(-50%, -50%);
  pointer-events: none;
}

.line {
  fill: none;
  stroke: var(--line-strong);
  stroke-width: 1;
  opacity: 0.7;
}

.line.dashed {
  stroke-dasharray: 2 10;
  stroke-linecap: round;
}

.tick {
  stroke: var(--line-strong);
  stroke-width: 1;
}

.dot {
  fill: var(--ink-3);
  opacity: 0.6;
}

.accent {
  fill: var(--accent);
}

.accent-halo {
  fill: var(--accent);
  opacity: 0.18;
  transform-box: fill-box;
  transform-origin: center;
  animation: halo 3.2s ease-in-out infinite;
}

/* every ring turns around the middle of the picture */
.ring {
  transform-origin: 600px 600px;
  animation: turn linear infinite;
}

.ring.reverse {
  animation-direction: reverse;
}

@keyframes turn {
  to {
    transform: rotate(360deg);
  }
}

/* the glow in the middle breathes slowly */
.glow {
  transform-origin: 600px 600px;
  animation: breathe 9s ease-in-out infinite;
}

@keyframes breathe {
  50% {
    transform: scale(1.08);
    opacity: 0.7;
  }
}

@keyframes halo {
  50% {
    transform: scale(1.6);
    opacity: 0;
  }
}
</style>
