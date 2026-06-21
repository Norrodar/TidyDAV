import { describe, it, expect } from 'vitest';
import { weekStartDate, weekStartISO, inWeek, nextDirection, flowKey } from './week';

// A fixed Wednesday for deterministic week math.
const wed = new Date(2026, 0, 14, 15, 30); // 2026-01-14, a Wednesday

describe('weekStartDate / weekStartISO', () => {
  it('snaps to Monday 00:00 of the current week', () => {
    expect(weekStartISO(0, wed)).toBe('2026-01-12'); // Monday
    const d = weekStartDate(0, wed);
    expect(d.getHours()).toBe(0);
    expect(d.getMinutes()).toBe(0);
  });

  it('offsets by whole weeks', () => {
    expect(weekStartISO(1, wed)).toBe('2026-01-19');
    expect(weekStartISO(-1, wed)).toBe('2026-01-05');
  });
});

describe('inWeek', () => {
  const start = weekStartDate(0, wed); // 2026-01-12

  // Local-time values kept away from window edges so the result is tz-independent.
  it('includes events inside the 7-day window', () => {
    expect(inWeek('2026-01-12T09:00:00', start)).toBe(true);
    expect(inWeek('2026-01-18T12:00:00', start)).toBe(true);
  });

  it('excludes events outside the window', () => {
    expect(inWeek('2026-01-11T09:00:00', start)).toBe(false);
    expect(inWeek('2026-01-19T12:00:00', start)).toBe(false);
  });

  it('always shows undated or unparseable events', () => {
    expect(inWeek('', start)).toBe(true);
    expect(inWeek('not-a-date', start)).toBe(true);
  });
});

describe('nextDirection / flowKey', () => {
  it('cycles a→b ⇒ b→a ⇒ bidirectional ⇒ a→b', () => {
    expect(nextDirection('a-to-b')).toBe('b-to-a');
    expect(nextDirection('b-to-a')).toBe('bidirectional');
    expect(nextDirection('bidirectional')).toBe('a-to-b');
  });

  it('maps each direction to its i18n key', () => {
    expect(flowKey('a-to-b')).toBe('flow_a_to_b');
    expect(flowKey('b-to-a')).toBe('flow_b_to_a');
    expect(flowKey('bidirectional')).toBe('flow_bidirectional');
  });
});
