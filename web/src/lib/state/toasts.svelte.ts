export type ToastKind = 'success' | 'error' | 'info';

interface Toast {
  id: number;
  message: string;
  kind: ToastKind;
}

/**
 * Cross-route toast singleton (Svelte 5 runes). Toasts auto-dismiss; they
 * survive client-side navigation because the store is mounted in the root layout.
 */
class ToastState {
  items = $state<Toast[]>([]);
  private seq = 0;

  show(message: string, kind: ToastKind = 'success') {
    const id = ++this.seq;
    this.items = [...this.items, { id, message, kind }];
    setTimeout(() => this.dismiss(id), 3500);
  }

  dismiss(id: number) {
    this.items = this.items.filter((t) => t.id !== id);
  }
}

export const toasts = new ToastState();
