/**
 * Promise-based confirmation dialog singleton (Svelte 5 runes), replacing the
 * native, un-themed window.confirm(). Mounted once via ConfirmDialog in the root
 * layout; `await confirmDialog.ask(...)` resolves true/false.
 */
class ConfirmState {
  open = $state(false);
  message = $state('');
  confirmLabel = $state('Confirm');
  private resolver: ((value: boolean) => void) | null = null;

  ask(message: string, confirmLabel = 'Confirm'): Promise<boolean> {
    this.message = message;
    this.confirmLabel = confirmLabel;
    this.open = true;
    return new Promise((resolve) => {
      this.resolver = resolve;
    });
  }

  private settle(value: boolean) {
    this.open = false;
    const resolve = this.resolver;
    this.resolver = null;
    resolve?.(value);
  }

  confirm() {
    this.settle(true);
  }
  cancel() {
    this.settle(false);
  }
}

export const confirmDialog = new ConfirmState();
