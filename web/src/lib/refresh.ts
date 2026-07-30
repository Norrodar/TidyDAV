// Formatting of the refresh interval TidyDAV publishes in the served ICS
// (REFRESH-INTERVAL / X-PUBLISHED-TTL). Kept out of the Svelte component so it
// can be unit-tested.

import { tf } from '$lib/i18n';

/** The i18n key and amount the calendar list badge should show for a TTL. */
export type RefreshBadge = { key: 'refresh_badge_hours' | 'refresh_badge_minutes'; n: number };

/**
 * Picks the badge wording: whole hours read as hours, everything else as
 * minutes. A sub-minute TTL still rounds up to one minute — the badge is only
 * rendered for ttlSeconds > 0, so "every 0 min" would be a lie.
 */
export function refreshBadgeParts(ttlSeconds: number): RefreshBadge {
  const minutes = Math.max(1, Math.round(ttlSeconds / 60));
  return minutes >= 60 && minutes % 60 === 0
    ? { key: 'refresh_badge_hours', n: minutes / 60 }
    : { key: 'refresh_badge_minutes', n: minutes };
}

/** Translated calendar list badge, e.g. "checks every 30 min". */
export function refreshBadge(ttlSeconds: number): string {
  const { key, n } = refreshBadgeParts(ttlSeconds);
  return tf(key, { n });
}
