import { describe, it, expect } from 'vitest';
import { webcalUrl, maskIcsUrl } from './share';

const SECRET = 'a3f9c1d2e4b6';

describe('webcalUrl', () => {
  it('swaps https for webcal', () => {
    expect(webcalUrl(`https://dav.example.com/ics/${SECRET}`)).toBe(
      `webcal://dav.example.com/ics/${SECRET}`
    );
  });

  it('swaps http for webcal', () => {
    expect(webcalUrl(`http://dav.example.com/ics/${SECRET}`)).toBe(
      `webcal://dav.example.com/ics/${SECRET}`
    );
  });

  it('keeps a non-default port', () => {
    expect(webcalUrl(`http://192.168.1.10:8080/ics/${SECRET}`)).toBe(
      `webcal://192.168.1.10:8080/ics/${SECRET}`
    );
  });

  it('keeps a sub-path base URL', () => {
    expect(webcalUrl(`https://example.com/tidy/ics/${SECRET}`)).toBe(
      `webcal://example.com/tidy/ics/${SECRET}`
    );
  });

  it('touches nothing but the scheme', () => {
    const url = `https://example.com:8443/tidy/ics/${SECRET}?x=1`;
    expect(webcalUrl(url)).toBe(url.replace('https:', 'webcal:'));
  });

  it('leaves an unknown scheme alone', () => {
    expect(webcalUrl(`webcal://example.com/ics/${SECRET}`)).toBe(
      `webcal://example.com/ics/${SECRET}`
    );
    expect(webcalUrl('not a url')).toBe('not a url');
  });
});

describe('maskIcsUrl', () => {
  it('keeps host and path but abbreviates the secret', () => {
    expect(maskIcsUrl(`https://dav.example.com/ics/${SECRET}`)).toBe(
      'https://dav.example.com/ics/a3f9…'
    );
  });

  it('works over plain http', () => {
    expect(maskIcsUrl(`http://dav.example.com/ics/${SECRET}`)).toBe(
      'http://dav.example.com/ics/a3f9…'
    );
  });

  it('keeps a non-default port', () => {
    expect(maskIcsUrl(`http://192.168.1.10:8080/ics/${SECRET}`)).toBe(
      'http://192.168.1.10:8080/ics/a3f9…'
    );
  });

  it('keeps a sub-path base URL', () => {
    expect(maskIcsUrl(`https://example.com/tidy/ics/${SECRET}`)).toBe(
      'https://example.com/tidy/ics/a3f9…'
    );
  });

  it('never shows the whole of a very short secret', () => {
    expect(maskIcsUrl('https://example.com/ics/abc')).toBe('https://example.com/ics/…');
  });

  it('shortens unparsable input instead of throwing', () => {
    expect(() => maskIcsUrl('')).not.toThrow();
    expect(maskIcsUrl('')).toBe('');
    const broken = maskIcsUrl('not-a-url-but-a-very-long-string-with-a-secret-in-it');
    expect(broken.endsWith('…')).toBe(true);
    expect(broken).not.toContain('secret-in-it');
  });

  it('does not leak the tail of the secret', () => {
    expect(maskIcsUrl(`https://dav.example.com/ics/${SECRET}`)).not.toContain('c1d2e4b6');
  });
});
