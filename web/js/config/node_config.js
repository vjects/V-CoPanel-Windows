/* ─── V-CoPanel Node.js Runtime Configuration Studio Module (node_config.js) ─── */

// Cache of runtime paths from DB: { 'node-v22': 'C:/...workspace.../node-v22', ... }
const _nodeRuntimePaths = {};

async function loadAvailableNodeVersions() {
	try {
		const res = await fetch('/api/runtimes/available');
		if (!res.ok) return;
		const all = await res.json();
		const nodeRuntimes = all.filter(r => r.key.startsWith('node-'));
		
		// Cache paths from DB
		nodeRuntimes.forEach(r => { _nodeRuntimePaths[r.key] = r.path || ''; });
		
		const verSel = document.getElementById('node_config-version-select');
		if (verSel) {
			const currentVal = verSel.value;
			let opts = `<option value="global">Global / Legacy Mode</option>`;
			nodeRuntimes.forEach(r => {
				opts += `<option value="${r.key}">${r.name || r.key}</option>`;
			});
			verSel.innerHTML = opts;
			
			if (currentVal && (currentVal === 'global' || nodeRuntimes.find(x => x.key === currentVal))) {
				verSel.value = currentVal;
			}
		}
		
		loadNodeStudio();
	} catch (e) {
		console.error('Failed to load Node versions:', e);
	}
}

async function loadNodeStudio() {
	const verSel = document.getElementById('node_config-version-select');
	let ver = verSel ? verSel.value : 'global';
	
	try {
		const res = await fetch('/api/nodeconfig/get?version=' + ver);
		if (!res.ok) return;
		const cfg = await res.json();

		const regSelect = document.getElementById('node_config-registry-select');
		const regUrl = document.getElementById('node_config-registry-url');
		const memSelect = document.getElementById('node_config-memory-select');

		if (memSelect && cfg.max_memory_mb) {
			memSelect.value = cfg.max_memory_mb.toString();
		}

		if (regSelect) {
			const known = [
				'https://registry.npmjs.org/',
				'https://registry.npmmirror.com/',
				'https://registry.yarnpkg.com/'
			];
			if (known.includes(cfg.registry)) {
				regSelect.value = cfg.registry;
				onNodeRegistrySelect();
			} else {
				regSelect.value = 'custom';
				onNodeRegistrySelect();
				if (regUrl) regUrl.value = cfg.registry;
			}
		}
	} catch (e) {
		console.error('Failed to load Node config:', e);
	}
}

function onNodeRegistrySelect() {
	const sel = document.getElementById('node_config-registry-select');
	const box = document.getElementById('node_config-custom-registry-box');
	if (sel && box) {
		if (sel.value === 'custom') {
			box.style.display = 'block';
		} else {
			box.style.display = 'none';
		}
	}
}

async function saveNodeConfig() {
	const verSel = document.getElementById('node_config-version-select');
	const sel = document.getElementById('node_config-registry-select');
	const urlInput = document.getElementById('node_config-registry-url');
	const memSel = document.getElementById('node_config-memory-select');

	let ver = verSel ? verSel.value : 'global';
	let reg = 'https://registry.npmjs.org/';
	if (sel) {
		reg = sel.value === 'custom' ? (urlInput ? urlInput.value.trim() : reg) : sel.value;
	}
	let mem = 4096;
	if (memSel) {
		mem = parseInt(memSel.value, 10) || 4096;
	}

	try {
		const res = await fetch('/api/nodeconfig/update?version=' + ver, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				registry: reg,
				max_memory_mb: mem,
				node_options: `--max-old-space-size=${mem}`
			})
		});
		if (!res.ok) {
			if (typeof notifier !== 'undefined' && notifier.error) notifier.error('Error', 'Failed to save Node configuration.');
			else if (typeof showToast === 'function') showToast('Error', 'Failed to save Node configuration.', 'error');
		}
	} catch (e) {
		console.error(e);
	}
}

async function restoreNodeDefaults() {
	if (!confirm('Are you sure you want to restore developer defaults for Node.js?')) return;
	const verSel = document.getElementById('node_config-version-select');
	let ver = verSel ? verSel.value : 'global';
	try {
		const res = await fetch('/api/nodeconfig/restore?version=' + ver, { method: 'POST' });
		if (res.ok) {
			loadNodeStudio();
		}
	} catch (e) {
		console.error(e);
	}
}

async function cleanNodeCache() {
	const verSel = document.getElementById('node_config-version-select');
	let ver = verSel ? verSel.value : 'global';
	const btn = document.getElementById('btn-clean-node_config-cache');
	if (btn) {
		btn.disabled = true;
		btn.innerHTML = '<span class="material-symbols-outlined" style="font-size:1.1rem; animation:spin 1s linear infinite;">sync</span> Cleaning...';
	}
	try {
		const res = await fetch('/api/nodeconfig/cache-clean?version=' + ver, { method: 'POST' });
		const data = await res.json();
		if (res.ok) {
			if (typeof notifier !== 'undefined' && notifier.success) notifier.success('NPM Cache Cleaned', 'Cache directory wiped successfully.');
		} else {
			if (typeof notifier !== 'undefined' && notifier.error) notifier.error('Clean Failed', data.error || 'Failed to clean NPM cache.');
		}
	} catch (e) {
		console.error(e);
	} finally {
		if (btn) {
			btn.disabled = false;
			btn.innerHTML = '<span class="material-symbols-outlined" style="font-size:1.1rem;">delete_sweep</span> Clean NPM Cache';
		}
	}
}
