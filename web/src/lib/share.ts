// Helpers for handing out a calendar's ICS link.
//
// The secret-id in that URL is the only thing guarding the calendar, so the UI
// treats it like a password: shown abbreviated by default, revealed on request.
// Both functions are pure so they can be unit-tested without a DOM.

/** How many characters of the secret stay visible in the masked form. */
const VISIBLE = 4;
const ELLIPSIS = '…';

/** Upper bound for the masked form of an input we could not parse. */
const FALLBACK_LENGTH = 24;

/**
 * Derives the `webcal://` variant of an ICS URL, which is what iOS, macOS and
 * Thunderbird need to offer a one-click subscription.
 *
 * Only the scheme is exchanged — host, port, path and query stay untouched. An
 * input that is not http(s) is returned unchanged.
 */
export function webcalUrl(icsUrl: string): string {
  return icsUrl.replace(/^https?:/i, 'webcal:');
}

/**
 * Shortens an ICS URL for display: scheme, host (incl. port) and the whole path
 * up to the last segment stay readable, the secret itself is cut to its first
 * few characters. `https://dav.example.com/ics/a3f9c1…`
 *
 * Anything unexpected — an unparsable URL, a secret too short to abbreviate —
 * is shortened more aggressively rather than shown in full. This never throws:
 * the calendar list must render even for a malformed link.
 */
export function maskIcsUrl(icsUrl: string): string {
  let parsed: URL;
  try {
    parsed = new URL(icsUrl);
  } catch {
    return clamp(icsUrl);
  }
  const cut = parsed.pathname.lastIndexOf('/');
  if (cut < 0) return clamp(icsUrl);

  const prefix = parsed.origin + parsed.pathname.slice(0, cut + 1);
  const secret = parsed.pathname.slice(cut + 1);
  // A secret this short would be given away completely by its first four
  // characters, so nothing of it is shown.
  if (secret.length <= VISIBLE) return prefix + ELLIPSIS;
  return prefix + secret.slice(0, VISIBLE) + ELLIPSIS;
}

/** Last-resort shortening for input we could not interpret as a URL. */
function clamp(value: string): string {
  return value.length > FALLBACK_LENGTH ? value.slice(0, FALLBACK_LENGTH) + ELLIPSIS : value;
}
