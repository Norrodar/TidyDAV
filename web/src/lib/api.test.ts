import { describe, it, expect } from 'vitest';
import { buildQuery, ApiError } from './api';

describe('buildQuery', () => {
  it('returns an empty string when there are no params', () => {
    expect(buildQuery({})).toBe('');
  });

  it('skips undefined, null and empty-string values', () => {
    expect(buildQuery({ a: undefined, b: null, c: '' })).toBe('');
  });

  it('encodes provided values with a leading question mark', () => {
    expect(buildQuery({ q: 'a b', n: 2, flag: true })).toBe('?q=a+b&n=2&flag=true');
  });
});

describe('ApiError', () => {
  it('is an Error that carries the status code and message', () => {
    const err = new ApiError(404, 'not found');
    expect(err).toBeInstanceOf(Error);
    expect(err.status).toBe(404);
    expect(err.message).toBe('not found');
    expect(err.name).toBe('ApiError');
  });
});
