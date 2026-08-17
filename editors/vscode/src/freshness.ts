/**
 * Two checks of one document can be in flight at once and finish in either
 * order; the slower one carries positions for text that has already changed.
 */
export class Freshness {
  private applied = new Map<string, number>();

  /** Equal versions pass, or a re-check after a settings change is dropped. */
  accept(key: string, version: number): boolean {
    const seen = this.applied.get(key);
    if (seen !== undefined && seen > version) {
      return false;
    }
    this.applied.set(key, version);
    return true;
  }

  forget(key: string): void {
    this.applied.delete(key);
  }
}
