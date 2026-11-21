export type ThemePref = 'system' | 'light' | 'dark';

const KEY = 'wbr-theme';

function read(): ThemePref {
	try {
		const v = localStorage.getItem(KEY);
		if (v === 'light' || v === 'dark') return v;
	} catch {
		// storage blocked
	}
	return 'system';
}

class ThemeStore {
	pref = $state<ThemePref>(typeof localStorage === 'undefined' ? 'system' : read());

	set(pref: ThemePref) {
		this.pref = pref;
		try {
			if (pref === 'system') localStorage.removeItem(KEY);
			else localStorage.setItem(KEY, pref);
		} catch {
			// storage blocked
		}
		if (pref === 'system') delete document.documentElement.dataset.theme;
		else document.documentElement.dataset.theme = pref;
	}

	cycle() {
		this.set(this.pref === 'system' ? 'light' : this.pref === 'light' ? 'dark' : 'system');
	}
}

export const theme = new ThemeStore();
