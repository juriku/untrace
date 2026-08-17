/**
 * untrace reports 1-based line and column where a column counts runes, which is
 * Unicode code points. VS Code positions are 0-based and count UTF-16 code
 * units, so an astral character occupies two of them. Tag characters and emoji
 * are astral and are exactly what untrace reports, so converting is not
 * optional: without it every marker after one on the same line lands short.
 */

/** Converts a 0-based rune index within `line` to a 0-based UTF-16 index. */
export function runeToUtf16(line: string, runeIndex: number): number {
  if (runeIndex <= 0) {
    return 0;
  }

  let runes = 0;
  let i = 0;
  while (i < line.length) {
    if (runes === runeIndex) {
      return i;
    }
    const cp = line.codePointAt(i);
    i += cp !== undefined && cp > 0xffff ? 2 : 1;
    runes++;
  }
  return line.length;
}

/** Counts the UTF-16 code units spanned by `runeCount` runes from `from`. */
export function runeSpanToUtf16(
  line: string,
  fromUtf16: number,
  runeCount: number,
): number {
  let runes = 0;
  let i = fromUtf16;
  while (i < line.length && runes < runeCount) {
    const cp = line.codePointAt(i);
    i += cp !== undefined && cp > 0xffff ? 2 : 1;
    runes++;
  }
  return i;
}
