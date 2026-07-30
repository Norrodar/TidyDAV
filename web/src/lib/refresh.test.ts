import { describe, it, expect } from 'vitest';
import { refreshBadge, refreshBadgeParts } from './refresh';

describe('refreshBadgeParts', () => {
  it('reads whole hours as hours', () => {
    expect(refreshBadgeParts(3600)).toEqual({ key: 'refresh_badge_hours', n: 1 });
    expect(refreshBadgeParts(7200)).toEqual({ key: 'refresh_badge_hours', n: 2 });
    expect(refreshBadgeParts(86400)).toEqual({ key: 'refresh_badge_hours', n: 24 });
  });

  it('reads anything else as minutes', () => {
    expect(refreshBadgeParts(900)).toEqual({ key: 'refresh_badge_minutes', n: 15 });
    expect(refreshBadgeParts(5400)).toEqual({ key: 'refresh_badge_minutes', n: 90 });
  });

  it('never reports a zero interval', () => {
    expect(refreshBadgeParts(30)).toEqual({ key: 'refresh_badge_minutes', n: 1 });
  });
});

describe('refreshBadge', () => {
  // Language-agnostic: only the interpolation is asserted, not the wording.
  it('interpolates the amount into the translated label', () => {
    expect(refreshBadge(7200)).toContain('2');
    expect(refreshBadge(7200)).not.toContain('{n}');
    expect(refreshBadge(900)).toContain('15');
  });
});
