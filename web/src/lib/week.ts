// Pure helpers shared by the calendar/sync preview panels: week windowing and
// sync-direction cycling. Kept separate from the Svelte components so they can be
// unit-tested.

import type { SyncDirection } from '$lib/api';

/** Monday 00:00 of the week `offset` weeks from `now`. */
export function weekStartDate(offset: number, now: Date = new Date()): Date {
  const d = new Date(now);
  d.setHours(0, 0, 0, 0);
  const monday = (d.getDay() + 6) % 7; // Mon = 0 … Sun = 6
  d.setDate(d.getDate() - monday + offset * 7);
  return d;
}

/** Local ISO date (YYYY-MM-DD) of weekStartDate(offset). */
export function weekStartISO(offset: number, now: Date = new Date()): string {
  const d = weekStartDate(offset, now);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

/**
 * Whether an event's ISO start falls in the 7-day window starting at `start`.
 * Undated or unparseable starts are always considered in-window (so events that
 * cannot be placed on a calendar are never hidden).
 */
export function inWeek(iso: string, start: Date): boolean {
  if (!iso) return true;
  const t = new Date(iso);
  if (isNaN(t.getTime())) return true;
  const end = new Date(start);
  end.setDate(end.getDate() + 7);
  return t >= start && t < end;
}

const directionOrder: SyncDirection[] = ['a-to-b', 'b-to-a', 'bidirectional'];

/** Next sync direction in the cycle a→b ⇒ b→a ⇒ bidirectional ⇒ a→b. */
export function nextDirection(d: SyncDirection): SyncDirection {
  const idx = directionOrder.indexOf(d);
  return directionOrder[(idx + 1) % directionOrder.length];
}

/** i18n key describing a sync direction. */
export function flowKey(d: SyncDirection): string {
  return d === 'b-to-a' ? 'flow_b_to_a' : d === 'bidirectional' ? 'flow_bidirectional' : 'flow_a_to_b';
}
