/* ─── V-CoPanel PHP Runtime Configuration Studio Module (php_config.js) ─── */

// Cache of runtime paths fetched from DB: { 'php-8.3': 'C:/...workspace.../php-8.3', ... }
const _phpRuntimePaths = {};

async function loadAvailablePHPVersions() {
	try {
		const res = await fetch('/api/runtimes/available');
		if (!res.ok) return;
		const all = await res.json();
		const phpRuntimes = all.filter(r => r.key.startsWith('php-'));
		
		// Cache paths from DB
		phpRuntimes.forEach(r => { _phpRuntimePaths[r.key] = r.path || ''; });
		
		const container = document.getElementById('php_config-tabs-container');
		if (container) {
			container.innerHTML = phpRuntimes.map(r => `
				<button class="pd-btn php-ver-tab" id="tab-${r.key}" onclick="selectPHPVersion('${r.key}')" style="height:34px; padding:0 14px; border:none; background:transparent;">${r.name || r.key}</button>
			`).join('');
			
			if (phpRuntimes.length > 0) {
				selectPHPVersion(phpRuntimes[0].key);
			}
		}
	} catch (e) {
		console.error('Failed to load PHP versions:', e);
	}
}

function selectPHPVersion(ver) {
	const hiddenInput = document.getElementById('php_config-version-select');
	if (hiddenInput) hiddenInput.value = ver;

	// Update visual tabs
	document.querySelectorAll('.php-ver-tab').forEach(btn => {
		btn.classList.remove('active');
		btn.classList.add('btn-outline');
		btn.style.background = 'transparent';
		btn.style.color = 'var(--text-main)';
		btn.style.border = 'none';
		btn.style.boxShadow = 'none';
	});

	const activeTab = document.getElementById('tab-' + ver);
	if (activeTab) {
		activeTab.classList.remove('btn-outline');
		activeTab.classList.add('active');
		activeTab.style.background = 'rgba(59,130,246,0.2)';
		activeTab.style.color = '#fff';
		activeTab.style.border = '1px solid rgba(96,165,250,0.5)';
		activeTab.style.boxShadow = '0 0 12px rgba(59,130,246,0.35)';
	}

	// Update status banners
	const verUpper = ver.toUpperCase().replace('-', ' ');
	const bannerTitle = document.getElementById('active-php-banner-title');
	if (bannerTitle) {
		bannerTitle.innerText = verUpper + ' Engine';
		bannerTitle.style.transition = 'opacity 0.2s';
		bannerTitle.style.opacity = '0.3';
		setTimeout(() => bannerTitle.style.opacity = '1', 150);
	}

	const bannerPath = document.getElementById('active-php-banner-path');
	if (bannerPath) {
		const installPath = _phpRuntimePaths[ver] || `workspace/runtimes/${ver}`;
		bannerPath.innerText = `${installPath}\\php.ini`;
	}

	const extBadge = document.getElementById('ext-badge-ver');
	if (extBadge) extBadge.innerText = verUpper;

	const saveBanner = document.getElementById('save-banner-ver');
	if (saveBanner) saveBanner.innerText = verUpper;

	const btnSave = document.getElementById('btn-save-php');
	if (btnSave) btnSave.innerHTML = `<span class="material-symbols-outlined">save</span> Save & Apply to ${verUpper}`;

	loadPHPConfig();
}

async function loadPHPConfig() {
	const verEl = document.getElementById('php_config-version-select');
	if (!verEl) return;
	const ver = verEl.value;
	const res = await fetch(`/api/phpini/get?ver=${ver}`);
	const data = await res.json();
	const setVal = (id, val) => { const el = document.getElementById(id); if (el) el.value = val; };
	setVal('cfg-memory', data.memory_limit || '512M');
	setVal('cfg-post', data.post_max_size || '128M');
	setVal('cfg-upload', data.upload_max_filesize || '128M');
	setVal('cfg-exec', data.max_execution_time || '300');
	setVal('cfg-input-time', data.max_input_time || '60');
	setVal('cfg-vars', data.max_input_vars || '3000');
	setVal('cfg-errors', data.display_errors || 'On');
	setVal('cfg-timezone', data.timezone || 'UTC');
	for (let ext in data.extensions) {
		const el = document.getElementById('ext-' + ext);
		if (el) el.checked = data.extensions[ext];
	}
}

async function savePHPConfig() {
	const verEl = document.getElementById('php_config-version-select');
	if (!verEl) return;
	const ver = verEl.value;
	const verUpper = ver.toUpperCase().replace('-', ' ');
	const extensions = {
		curl: document.getElementById('ext-curl')?.checked || false,
		mbstring: document.getElementById('ext-mbstring')?.checked || false,
		openssl: document.getElementById('ext-openssl')?.checked || false,
		pdo_mysql: document.getElementById('ext-pdo_mysql')?.checked || false,
		mysqli: document.getElementById('ext-mysqli')?.checked || false,
		fileinfo: document.getElementById('ext-fileinfo')?.checked || false,
		gd: document.getElementById('ext-gd')?.checked || false,
		zip: document.getElementById('ext-zip')?.checked || false,
		intl: document.getElementById('ext-intl')?.checked || false,
		exif: document.getElementById('ext-exif')?.checked || false,
		bcmath: document.getElementById('ext-bcmath')?.checked || false,
		soap: document.getElementById('ext-soap')?.checked || false,
		sockets: document.getElementById('ext-sockets')?.checked || false,
		pdo_sqlite: document.getElementById('ext-pdo_sqlite')?.checked || false
	};
	const res = await fetch('/api/phpini/update', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			version: ver,
			config: {
				memory_limit: document.getElementById('cfg-memory').value,
				post_max_size: document.getElementById('cfg-post').value,
				upload_max_filesize: document.getElementById('cfg-upload').value,
				max_execution_time: document.getElementById('cfg-exec').value,
				max_input_time: document.getElementById('cfg-input-time').value,
				max_input_vars: document.getElementById('cfg-vars').value,
				display_errors: document.getElementById('cfg-errors').value,
				timezone: document.getElementById('cfg-timezone').value,
				extensions: extensions
			}
		})
	});
	if (!res.ok) {
		const errText = await res.text();
		if (typeof showToast === 'function') showToast(`Error saving ${verUpper}`, errText, 'error');
		return;
	}
	const dynamicPath = _phpRuntimePaths[ver] ? `${_phpRuntimePaths[ver]}\\php.ini` : `${ver}\\php.ini`;
	if (typeof showToast === 'function') showToast('Configuration Saved', `${verUpper} settings written to ${dynamicPath}`, 'success');
}

async function restorePHPConfig() {
	const verEl = document.getElementById('php_config-version-select');
	if (!verEl) return;
	const ver = verEl.value;
	const verUpper = ver.toUpperCase().replace('-', ' ');
	if (!confirm(`Reset ${verUpper} PHP engine settings to factory defaults? This will overwrite your current php.ini.`)) return;
	const res = await fetch('/api/phpini/restore', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ version: ver })
	});
	if (!res.ok) {
		const errText = await res.text();
		if (typeof showToast === 'function') showToast(`Restore Failed for ${verUpper}`, errText, 'error');
		return;
	}
	if (typeof showToast === 'function') showToast('Factory Defaults Restored', `${verUpper} engine settings reset to default values.`, 'success');
	loadPHPConfig();
}
