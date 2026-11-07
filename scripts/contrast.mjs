#!/usr/bin/env node
// Contrast gate: computes WCAG contrast for every token pair the UI uses for
// text (foreground colors on the four base backgrounds, semantic colors on
// their own soft background, text on soft backgrounds) and fails below 4.5:1
// in both themes. Reads tokens.css directly so drift cannot slip through.
import { readFile } from "node:fs/promises";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");
const css = await readFile(path.join(root, "web", "src", "styles", "tokens.css"), "utf8");

function parseTheme(block) {
  const values = {};
  for (const [, name, value] of block.matchAll(/(--[a-z0-9-]+):\s*(#[0-9a-fA-F]{6})/g)) {
    values[name] = value;
  }
  return values;
}

const darkMatch = css.match(/:root,\n:root\[data-theme="dark"\]\s*\{([\s\S]*?)\n\}/);
const lightMatch = css.match(/:root\[data-theme="light"\]\s*\{([\s\S]*?)\n\}/);
if (!darkMatch || !lightMatch) {
  console.error("contrast: could not locate both theme blocks in tokens.css");
  process.exit(1);
}

const luminance = (hex) => {
  const channels = [1, 3, 5].map((offset) => {
    const raw = parseInt(hex.slice(offset, offset + 2), 16) / 255;
    return raw <= 0.03928 ? raw / 12.92 : ((raw + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
};

const contrast = (fg, bg) => {
  const l1 = luminance(fg);
  const l2 = luminance(bg);
  return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
};

const baseBackgrounds = ["--bg", "--bg-panel", "--bg-raised", "--bg-input"];
const foregrounds = [
  "--text",
  "--text-dim",
  "--text-faint",
  "--accent",
  "--blocked",
  "--recorded",
  "--ratelimited",
  "--ok",
];
const softPairs = [
  ["--blocked", "--blocked-soft"],
  ["--recorded", "--recorded-soft"],
  ["--ratelimited", "--ratelimited-soft"],
  ["--ok", "--ok-soft"],
];

const failures = [];
for (const [theme, block] of [
  ["dark", parseTheme(darkMatch[1])],
  ["light", parseTheme(lightMatch[1])],
]) {
  const pairs = [];
  for (const fg of foregrounds) {
    for (const bg of baseBackgrounds) {
      pairs.push([fg, bg]);
    }
  }
  for (const [fg, bg] of softPairs) {
    pairs.push([fg, bg]);
    pairs.push(["--text", bg]);
  }
  for (const [fg, bg] of pairs) {
    const ratio = contrast(block[fg], block[bg]);
    if (ratio < 4.5) {
      failures.push(`${theme}: ${fg} on ${bg} = ${ratio.toFixed(2)}:1`);
    }
  }
}

if (failures.length > 0) {
  console.error("contrast gate failed:");
  for (const failure of failures) {
    console.error(`  - ${failure}`);
  }
  process.exit(1);
}
console.log("contrast: all checked token pairs pass 4.5:1 in both themes");
