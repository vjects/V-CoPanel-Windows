/* ─── V-CoPanel Go Runtime Configuration Studio Module (go_config.js) ─── */

// Cache of runtime paths from DB: { 'go': 'C:/...workspace.../go', ... }
const _goRuntimePaths = {};

async function loadAvailableGoVersions() {
	try {
		const res = await fetch('/api/runtimes/available');
		if (!res.ok) return;
		const all = await res.json();
		const goRuntimes = all.filter(r => r.key.startsWith('go') || r.key === 'go');
		
		// Cache paths from DB
		goRuntimes.forEach(r => { _goRuntimePaths[r.key] = r.path || ''; });
		
		const verSel = document.getElementById('go_config-version-select');
		if (verSel) {
			const currentVal = verSel.value;
			let opts = `<option value="global">Golang Global SDK</option>`;
			goRuntimes.forEach(r => {
				opts += `<option value="${r.key}">${r.name || r.key}</option>`;
			});
			verSel.innerHTML = opts;
			
			if (currentVal && (currentVal === 'global' || goRuntimes.find(x => x.key === currentVal))) {
				verSel.value = currentVal;
			}
		}
		
		loadGoStudio();
	} catch (e) {
		console.error('Failed to load Go versions:', e);
	}
}

async function loadGoStudio() {
	const verSel = document.getElementById('go_config-version-select');
	let ver = verSel ? verSel.value : 'global';
	try {
		const res = await fetch('/api/goconfig/get?version=' + ver);
		if (!res.ok) return;
		const cfg = await res.json();

		const proxySelect = document.getElementById('go_config-proxy-select');
		const proxyVal = document.getElementById('go_config-proxy-val');
		const cgoSelect = document.getElementById('go_config-cgo-select');

		if (cgoSelect && cfg.cgo_enabled !== undefined) {
			cgoSelect.value = cfg.cgo_enabled;
		}

		if (proxySelect) {
			const known = [
				'https://proxy.golang.org,direct',
				'https://goproxy.io,direct',
				'https://goproxy.cn,direct',
				'direct'
			];
			if (known.includes(cfg.goproxy)) {
				proxySelect.value = cfg.goproxy;
				onGoProxySelect();
			} else {
				proxySelect.value = 'custom';
				onGoProxySelect();
				if (proxyVal) proxyVal.value = cfg.goproxy;
			}
		}
	} catch (e) {
		console.error('Failed to load Go config:', e);
	}
}

function onGoProxySelect() {
	const sel = document.getElementById('go_config-proxy-select');
	const box = document.getElementById('go_config-custom-proxy-box');
	if (sel && box) {
		if (sel.value === 'custom') {
			box.style.display = 'block';
		} else {
			box.style.display = 'none';
		}
	}
}

async function saveGoConfig() {
	const sel = document.getElementById('go_config-proxy-select');
	const proxyVal = document.getElementById('go_config-proxy-val');
	const cgoSel = document.getElementById('go_config-cgo-select');

	let proxy = 'https://proxy.golang.org,direct';
	if (sel) {
		proxy = sel.value === 'custom' ? (proxyVal ? proxyVal.value.trim() : proxy) : sel.value;
	}
	let cgo = '0';
	if (cgoSel) {
		cgo = cgoSel.value;
	}

	const verSel = document.getElementById('go_config-version-select');
	let ver = verSel ? verSel.value : 'global';

	try {
		const res = await fetch('/api/goconfig/update?version=' + ver, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				goproxy: proxy,
				cgo_enabled: cgo
			})
		});
		if (!res.ok) {
			if (typeof notifier !== 'undefined' && notifier.error) notifier.error('Error', 'Failed to save Go configuration.');
			else if (typeof showToast === 'function') showToast('Error', 'Failed to save Go configuration.', 'error');
		}
	} catch (e) {
		console.error(e);
	}
}

async function restoreGoDefaults() {
	if (!confirm('Are you sure you want to restore developer defaults for Go?')) return;
	const verSel = document.getElementById('go_config-version-select');
	let ver = verSel ? verSel.value : 'global';
	try {
		const res = await fetch('/api/goconfig/restore?version=' + ver, { method: 'POST' });
		if (res.ok) {
			if (typeof showToast === 'function') {
				showToast('Defaults Restored', 'Go settings reset to optimal developer defaults.', 'success');
			}
			loadGoStudio();
		}
	} catch (e) {
		console.error(e);
	}
}

async function cleanGoCache() {
	const btn = document.getElementById('btn-clean-go_config-cache');
	if (btn) {
		btn.disabled = true;
		btn.innerHTML = '<span class="material-symbols-outlined" style="font-size:1.1rem; animation:spin 1s linear infinite;">sync</span> Cleaning...';
	}
	const verSel = document.getElementById('go_config-version-select');
	let ver = verSel ? verSel.value : 'global';
	try {
		const res = await fetch('/api/goconfig/cache-clean?version=' + ver, { method: 'POST' });
		const data = await res.json();
		if (res.ok) {
			if (typeof notifier !== 'undefined' && notifier.success) notifier.success('Go Cache Cleaned', 'Build and module caches wiped successfully.');
		} else {
			if (typeof notifier !== 'undefined' && notifier.error) notifier.error('Clean Failed', data.error || 'Failed to clean Go cache.');
		}
	} catch (e) {
		console.error(e);
	} finally {
		if (btn) {
			btn.disabled = false;
			btn.innerHTML = '<span class="material-symbols-outlined" style="font-size:1.1rem;">delete_sweep</span> Clean Go Cache';
		}
	}
}
