// Builds the colour palettes (Midnight, Forest, Plum, Mocha, AMOLED).
//
// The interface was written with dark colours straight in the stylesheets, so
// instead of rewriting them this script reads every stylesheet and, for each
// rule that uses a dark neutral colour, writes the same rule again under
// html[data-palette="<id>"] with the colour re-tinted. Vivid colours (accent,
// green, red, ...) and the light theme are left alone.
//
// Run:  npm run gen:palettes     (the result is committed)
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import postcss from 'postcss';

const here = path.dirname(fileURLToPath(import.meta.url));
const src = path.join(here, '..', 'src');

export const PALETTES = [
  { id: 'midnight', name: 'Midnight', hue: 222, sat: 0.34, light: 1 },
  { id: 'forest', name: 'Forest', hue: 152, sat: 0.2, light: 1 },
  { id: 'plum', name: 'Plum', hue: 282, sat: 0.24, light: 1 },
  { id: 'mocha', name: 'Mocha', hue: 28, sat: 0.2, light: 1 },
  { id: 'amoled', name: 'AMOLED', hue: 0, sat: 0, light: 0.45 },
];

// ---------- colour maths ----------
const clamp = (v, a, b) => Math.max(a, Math.min(b, v));

function hexToRgb(h) {
  let s = h.slice(1);
  if (s.length === 3 || s.length === 4) s = s.split('').map(c => c + c).join('');
  const n = parseInt(s.slice(0, 6), 16);
  const a = s.length === 8 ? parseInt(s.slice(6), 16) / 255 : 1;
  return { r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255, a };
}

function rgbToHsl({ r, g, b }) {
  r /= 255; g /= 255; b /= 255;
  const max = Math.max(r, g, b), min = Math.min(r, g, b);
  const l = (max + min) / 2;
  let h = 0, s = 0;
  if (max !== min) {
    const d = max - min;
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
    if (max === r) h = (g - b) / d + (g < b ? 6 : 0);
    else if (max === g) h = (b - r) / d + 2;
    else h = (r - g) / d + 4;
    h *= 60;
  }
  return { h, s, l };
}

function hslToRgb({ h, s, l }) {
  const c = (1 - Math.abs(2 * l - 1)) * s;
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
  const m = l - c / 2;
  let [r, g, b] = [0, 0, 0];
  if (h < 60) [r, g, b] = [c, x, 0];
  else if (h < 120) [r, g, b] = [x, c, 0];
  else if (h < 180) [r, g, b] = [0, c, x];
  else if (h < 240) [r, g, b] = [0, x, c];
  else if (h < 300) [r, g, b] = [x, 0, c];
  else [r, g, b] = [c, 0, x];
  return { r: (r + m) * 255, g: (g + m) * 255, b: (b + m) * 255 };
}

const toHex = ({ r, g, b }) =>
  '#' + [r, g, b].map(v => clamp(Math.round(v), 0, 255).toString(16).padStart(2, '0')).join('');

// Returns the re-tinted colour, or null when the colour should stay as it is.
export function retint(rgb, p) {
  const { h: _h, s, l } = rgbToHsl(rgb);
  if (l < 0.03) return null; // shadows and pure black
  if (l > 0.78) return null; // white text and highlights
  if (s > 0.3) return null; // accent, green, red, yellow, the purple login screen ...
  // surfaces take the full tint; lighter greys (secondary text) only a little
  const k = clamp((0.72 - l) / 0.42, 0.25, 1);
  const out = hslToRgb({ h: p.hue, s: p.sat * k, l: clamp(l * (l < 0.45 ? p.light : 1), 0, 1) });
  return out;
}

const HEX = /#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3,4})\b/g;
const RGBA = /rgba?\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*(?:,\s*([\d.]+)\s*)?\)/g;

export function retintValue(value, p) {
  let changed = false;
  let out = value.replace(HEX, m => {
    const rgb = hexToRgb(m);
    const t = retint(rgb, p);
    if (!t) return m;
    changed = true;
    const hex = toHex(t);
    return rgb.a < 1 ? hex + Math.round(rgb.a * 255).toString(16).padStart(2, '0') : hex;
  });
  out = out.replace(RGBA, (m, r, g, b, a) => {
    const rgb = { r: +r, g: +g, b: +b };
    const t = retint(rgb, p);
    if (!t) return m;
    changed = true;
    const R = Math.round(t.r), G = Math.round(t.g), B = Math.round(t.b);
    return a === undefined ? `rgb(${R}, ${G}, ${B})` : `rgba(${R}, ${G}, ${B}, ${a})`;
  });
  return changed ? out : null;
}

// ---------- stylesheets ----------
// In the order the browser applies them (the order of the built bundle). A rule
// of a later file beats an earlier one, so the palette rules must keep that
// order too. palettes.generated.css itself is loaded last (see main.tsx).
const files = [
  'App.css',
  'styles/server.css',
  'styles/presence.css',
  'components/Markdown.css',
  'components/Calendar.css',
  'styles/theme-picker.css',
  'styles/decisions.css',
  'styles/polish.css',
  'styles/thread.css',
  'styles/pins.css',
  'styles/mobile.css',
  'styles/thread-extras.css',
];

const scope = id => `html[data-palette="${id}"]:not([data-theme="light"])`;
// :where() adds no specificity, so a palette rule weighs exactly what the rule it
// copies weighed and the cascade between rules keeps working as before.
const ruleScope = id => `:where(${scope(id)})`;

function prefixSelector(sel, id) {
  const pre = ruleScope(id);
  return sel
    .split(',')
    .map(s => s.trim())
    .filter(Boolean)
    .map(s => {
      if (s === ':root' || s === 'html') return pre;
      if (/^(:root|html)\b/.test(s)) return pre + s.replace(/^(:root|html)/, '');
      return `${pre} ${s}`;
    })
    .join(',\n');
}

const inKeyframes = node => {
  for (let p = node.parent; p; p = p.parent) if (p.type === 'atrule' && /keyframes$/i.test(p.name)) return true;
  return false;
};
const atRuleChain = node => {
  const chain = [];
  for (let p = node.parent; p && p.type === 'atrule'; p = p.parent) chain.unshift(p);
  return chain;
};
// the light theme has its own rules; the sign-in screen keeps its brand colours
const isLightRule = rule => rule.selector.includes('[data-theme') || /(^|[\s,])\.login/.test(rule.selector);

const normSel = rule => rule.selector.replace(/\s+/g, ' ').trim();
const mediaKey = rule => atRuleChain(rule).map(a => `@${a.name} ${a.params}`).join('|');
const declKey = (rule, prop) => `${mediaKey(rule)}|${normSel(rule)}|${prop}`;

// Every stylesheet in cascade order, parsed once.
export function parseAll(sheets) {
  return sheets.map(css => postcss.parse(css));
}

export function buildPaletteCss(roots, p) {
  // Pass 1: which (selector, property) pairs get a tinted value somewhere.
  const tinted = new Set();
  for (const root of roots) {
    root.walkRules(rule => {
      if (inKeyframes(rule) || isLightRule(rule)) return;
      rule.walkDecls(d => {
        if (d.parent === rule && retintValue(d.value, p) !== null) tinted.add(declKey(rule, d.prop));
      });
    });
  }
  // Pass 2: write them in order. When a later rule changes such a property back
  // to something that is not tinted (e.g. transparent), write that value too, or
  // the palette rule written for the earlier file would win over it.
  const rules = [];
  for (const root of roots) {
    root.walkRules(rule => {
      if (inKeyframes(rule) || isLightRule(rule)) return;
      const decls = [];
      rule.walkDecls(d => {
        if (d.parent !== rule) return;
        const t = retintValue(d.value, p);
        const value = t !== null ? t : tinted.has(declKey(rule, d.prop)) ? d.value : null;
        if (value !== null) decls.push(`  ${d.prop}: ${value}${d.important ? ' !important' : ''};`);
      });
      if (!decls.length) return;
      let block = `${prefixSelector(rule.selector, p.id)} {\n${decls.join('\n')}\n}`;
      for (const at of atRuleChain(rule).reverse()) {
        block = `@${at.name} ${at.params} {\n${block.replace(/^/gm, '  ')}\n}`;
      }
      rules.push(block);
    });
  }
  return rules.join('\n\n');
}

// The variables the newer components read, tinted the same way.
const VARS = {
  '--bg-primary': '#1e1f22',
  '--bg-secondary': '#2b2d31',
  '--bg-tertiary': '#313338',
  '--border-color': '#3f4147',
  '--text-secondary': '#8a8f98',
};

function varsBlock(p) {
  const lines = Object.entries(VARS).map(([k, v]) => {
    const t = retint(hexToRgb(v), p);
    return `  ${k}: ${t ? toHex(t) : v};`;
  });
  return `${scope(p.id)} {\n${lines.join('\n')}\n}`;
}

export function swatches(p) {
  const pick = hex => {
    const t = retint(hexToRgb(hex), p);
    return t ? toHex(t) : hex;
  };
  return { bg: pick('#1e1f22'), panel: pick('#2b2d31'), line: pick('#3f4147') };
}

function main() {
  const roots = parseAll(files.map(f => fs.readFileSync(path.join(src, f), 'utf8')));
  const parts = ['/* Generated by scripts/gen-palettes.mjs. Do not edit by hand. */'];
  for (const p of PALETTES) {
    parts.push(`/* ${p.name} */`, varsBlock(p), buildPaletteCss(roots, p));
  }
  fs.writeFileSync(path.join(src, 'styles', 'palettes.generated.css'), parts.join('\n\n') + '\n');

  const meta = PALETTES.map(p => ({ id: p.id, name: p.name, ...swatches(p) }));
  fs.writeFileSync(
    path.join(src, 'components', 'palettes.generated.ts'),
    `// Generated by scripts/gen-palettes.mjs. Do not edit by hand.\nexport interface PaletteInfo {\n  id: string;\n  name: string;\n  bg: string;\n  panel: string;\n  line: string;\n}\n\nexport const GENERATED_PALETTES: PaletteInfo[] = ${JSON.stringify(meta, null, 2)};\n`
  );
  console.log('palettes written:', PALETTES.map(p => p.id).join(', '));
}

if (process.argv[1] === fileURLToPath(import.meta.url)) main();
