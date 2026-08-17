import assert from "node:assert/strict";
import { test } from "node:test";

import { runeSpanToUtf16, runeToUtf16 } from "./positions.ts";

const tagT = String.fromCodePoint(0xe0074);
const zwj = String.fromCodePoint(0x200d);
const family = `\u{1F468}${zwj}\u{1F469}${zwj}\u{1F467}`;

test("ascii maps one to one", () => {
  assert.equal(runeToUtf16("hello", 0), 0);
  assert.equal(runeToUtf16("hello", 3), 3);
});

test("an astral character occupies two utf-16 units", () => {
  // "a" then U+1D400, so the rune at index 2 sits at UTF-16 index 3.
  const line = "a\u{1D400}b";
  assert.equal(runeToUtf16(line, 1), 1);
  assert.equal(runeToUtf16(line, 2), 3);
});

test("tag characters shift every later column", () => {
  const line = "x" + tagT + tagT + "y";
  assert.equal(runeToUtf16(line, 1), 1);
  assert.equal(runeToUtf16(line, 3), 5);
});

test("an emoji sequence counts joiners as runes", () => {
  const line = family + "!";
  // Three astral faces and two joiners, so the "!" is the sixth rune.
  assert.equal(runeToUtf16(line, 5), 8);
});

test("indices past the end clamp to the line length", () => {
  assert.equal(runeToUtf16("ab", 99), 2);
  assert.equal(runeToUtf16("", 3), 0);
});

test("negative and zero indices clamp to zero", () => {
  assert.equal(runeToUtf16("abc", -1), 0);
  assert.equal(runeToUtf16("abc", 0), 0);
});

test("a span of astral runes measures in utf-16 units", () => {
  const line = "go " + tagT + tagT + tagT + " end";
  const start = runeToUtf16(line, 3);
  assert.equal(start, 3);
  assert.equal(runeSpanToUtf16(line, start, 3), 9);
});

test("a span stops at the end of the line", () => {
  assert.equal(runeSpanToUtf16("ab", 0, 99), 2);
});

test("a lone surrogate does not loop forever", () => {
  const line = "a\uD800b";
  assert.equal(runeToUtf16(line, 2), 2);
});
