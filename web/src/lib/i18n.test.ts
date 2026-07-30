import { describe, it, expect } from 'vitest';
import { dictionaries } from './i18n';

// A key present in only one language silently falls back to English, which
// looks like a bug to German users and is easy to miss in review.
describe('translation tables', () => {
  const en = dictionaries.en;
  const de = dictionaries.de;

  it('translates every English key into German', () => {
    const missing = Object.keys(en).filter((k) => !(k in de));
    expect(missing).toEqual([]);
  });

  it('has no German key without an English original', () => {
    const extra = Object.keys(de).filter((k) => !(k in en));
    expect(extra).toEqual([]);
  });

  it('leaves no value empty', () => {
    for (const [name, dict] of Object.entries(dictionaries)) {
      const empty = Object.entries(dict)
        .filter(([, v]) => v.trim() === '')
        .map(([k]) => `${name}.${k}`);
      expect(empty).toEqual([]);
    }
  });

  it('keeps the same {placeholders} in both languages', () => {
    const tokens = (s: string) => (s.match(/\{[a-zA-Z]+\}/g) ?? []).sort();
    for (const key of Object.keys(en)) {
      if (!(key in de)) continue;
      expect({ key, tokens: tokens(de[key]) }).toEqual({ key, tokens: tokens(en[key]) });
    }
  });
});
