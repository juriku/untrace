import assert from "node:assert/strict";
import { test } from "node:test";

import { Freshness } from "./freshness.ts";

const DOC = "file:///a.md";

test("the first result for a document is applied", () => {
  const f = new Freshness();
  assert.equal(f.accept(DOC, 1), true);
});

test("a newer result is applied after an older one", () => {
  const f = new Freshness();
  f.accept(DOC, 1);
  assert.equal(f.accept(DOC, 2), true);
});

test("a result that finishes after a newer one is dropped", () => {
  const f = new Freshness();
  assert.equal(f.accept(DOC, 5), true);
  assert.equal(f.accept(DOC, 4), false);
});

test("the same version is applied again", () => {
  const f = new Freshness();
  f.accept(DOC, 3);
  assert.equal(f.accept(DOC, 3), true);
});

test("documents are tracked independently", () => {
  const f = new Freshness();
  f.accept(DOC, 9);
  assert.equal(f.accept("file:///b.md", 1), true);
});

test("a reopened document starts over", () => {
  const f = new Freshness();
  f.accept(DOC, 9);
  f.forget(DOC);
  assert.equal(f.accept(DOC, 1), true);
});
