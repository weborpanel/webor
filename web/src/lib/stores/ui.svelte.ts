/** Shell-level UI state that pages can toggle (modals owned by the layout). */
class UiStore {
	addSiteOpen = $state(false);
	paletteOpen = $state(false);
	drawerOpen = $state(false);
}

export const ui = new UiStore();
