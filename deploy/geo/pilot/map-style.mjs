// Run after pinned npm install into MAP_DATA_DIR/style-build (outside Git).
import { createRequire } from 'node:module'
import { mkdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
const dir = resolve(process.argv[2])
const origin = process.argv[3] || 'http://127.0.0.1:8092'
const require = createRequire(`${dir}/style-build/package.json`)
const { layers, namedFlavor } = require('@protomaps/basemaps')
mkdirSync(`${dir}/styles`, { recursive: true })
for (const theme of ['light', 'dark']) {
  const dark = theme === 'dark'
  const bg = dark ? '#0e1724' : '#f5f6f8'
  const fg = dark ? '#e9eff8' : '#172538'
  const muted = dark ? '#213047' : '#edf1f6'
  const flavor = { ...namedFlavor(theme), background: bg, earth: bg,
    buildings: muted, water: dark ? '#152d4e' : '#cedfff',
    city_label: fg, subplace_label: fg, roads_label_major: fg,
    city_label_halo: bg, subplace_label_halo: bg, roads_label_major_halo: bg }
  const style = { version: 8, name: `Corpord ${theme} — Екатеринбург`,
    metadata: { 'corpord:coverage': [60.45,56.72,60.75,56.92],
      'corpord:primary': dark ? '#6d9aff' : '#245ce3',
      'corpord:source-date': '2026-10-31', 'corpord:basemaps-version': '5.7.2' },
    center: [60.60,56.835], zoom: 12,
    glyphs: `${origin}/fonts/{fontstack}/{range}.pbf`,
    sprite: `${origin}/sprites/v4/${theme}`,
    sources: { protomaps: { type: 'vector', url: `pmtiles://${origin}/yekaterinburg.pmtiles`,
      attribution: '<a href="https://protomaps.com">Protomaps</a> © <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> | <a href="https://esa-worldcover.org/en/data-access">ESA WorldCover</a> (<a href="https://creativecommons.org/licenses/by/4.0/">CC BY 4.0</a>)',
      bounds: [60.45,56.72,60.75,56.92], minzoom: 0, maxzoom: 14 } },
    layers: layers('protomaps', flavor, { lang: 'ru' }) }
  writeFileSync(`${dir}/styles/corpord-${theme}.json`, JSON.stringify(style, null, 2)+'\n')
}
