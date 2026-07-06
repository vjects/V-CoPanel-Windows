function openAdvancedScaffoldModal() {
	const modal = document.getElementById('scaffold-modal');
	const formBox = document.getElementById('scaffold-form-content');
	const loadingBox = document.getElementById('scaffold-loading-state');
	if (modal) {
		if (formBox) formBox.style.display = 'block';
		if (loadingBox) loadingBox.style.display = 'none';
		modal.style.display = 'flex';
		const nameInput = document.getElementById('scaffold-name');
		if (nameInput) {
			nameInput.value = '';
			nameInput.focus();
		}
		onStackChanged();
		updateScaffoldPathPreview();
		populateAvailableRuntimes();
	}
}

async function populateAvailableRuntimes() {
	try {
		const res = await fetch('/api/runtimes/available');
		if (!res.ok) return;
		const list = await res.json();
		if (!list || list.length === 0) return;

		const phpSel = document.getElementById('scaffold-php-ver');
		const nodeSel = document.getElementById('scaffold-node-ver');
		const goSel = document.getElementById('scaffold-go-ver');

		const phps = list.filter(r => r.key.startsWith('php-'));
		const nodes = list.filter(r => r.key.startsWith('node-'));
		const gos = list.filter(r => r.key.startsWith('go-'));

		if (phpSel && phps.length > 0) {
			phpSel.innerHTML = phps.map(r => `<option value="${r.key}">${r.name} (${r.version})</option>`).join('');
		}
		if (nodeSel && nodes.length > 0) {
			nodeSel.innerHTML = nodes.map(r => `<option value="${r.key}">${r.name} (${r.version})</option>`).join('');
		}
		if (goSel && gos.length > 0) {
			goSel.innerHTML = gos.map(r => `<option value="${r.key}">${r.name} (${r.version})</option>`).join('');
		}
	} catch (e) {}
}

function closeScaffoldModal() {
	const modal = document.getElementById('scaffold-modal');
	if (modal) modal.style.display = 'none';
}

function updateScaffoldPathPreview() {
	const previewEl = document.getElementById('scaffold-target-path-preview');
	if (!previewEl) return;
	const wsInput = document.getElementById('workspace-input');
	let base = wsInput ? wsInput.value.trim() : 'C:/Projects';
	base = base.replace(/\\/g, '/').replace(/\/$/, '');
	if (base.endsWith('/workspace') || base === 'workspace') {
		base = base + '/projects';
	}
	const name = document.getElementById('scaffold-name')?.value.trim() || 'my-awesome-app';
	previewEl.textContent = `${base}/${name}`;
}

function validateProjectNameInput(input) {
	const val = input.value;
	const clean = val.replace(/[^a-zA-Z0-9\-_]/g, '');
	if (val !== clean) {
		input.value = clean;
	}
	const errEl = document.getElementById('scaffold-name-error');
	if (errEl) {
		errEl.style.display = val !== clean ? 'block' : 'none';
	}
	updateScaffoldPathPreview();
}

function onStackChanged() {
	const stack = document.getElementById('scaffold-stack')?.value || 'laravel';
	const phpBox = document.getElementById('runtime-php-box');
	const nodeBox = document.getElementById('runtime-node-box');
	const goBox = document.getElementById('runtime-go-box');
	const portInput = document.getElementById('scaffold-port');

	if (phpBox) phpBox.style.display = 'block';
	if (nodeBox) nodeBox.style.display = 'block';
	if (goBox) goBox.style.display = 'block';

	if (portInput) {
		if (stack === 'node') portInput.value = '3000';
		else if (stack === 'go') portInput.value = '8080';
		else if (stack === 'empty') portInput.value = '8000';
		else portInput.value = '8000';
	}
}

async function submitAdvancedScaffold() {
	const nameInput = document.getElementById('scaffold-name');
	const name = nameInput?.value.trim();
	const stack = document.getElementById('scaffold-stack')?.value || 'laravel';
	const port = document.getElementById('scaffold-port')?.value || '8000';
	const collation = document.getElementById('scaffold-collation')?.value || 'utf8mb4_unicode_ci';
	let php_version = document.getElementById('scaffold-php-ver')?.value || '';
	let node_version = document.getElementById('scaffold-node-ver')?.value || '';
	let go_version = document.getElementById('scaffold-go-ver')?.value || '';
	


	if (!name) {
		if (typeof showToast === 'function') showToast('Missing Name', 'Please enter a project directory name.', 'warning');
		return;
	}
	if (!/^[a-zA-Z0-9\-_]+$/.test(name)) {
		if (typeof showToast === 'function') showToast('Invalid Name Format', 'Use only English letters, numbers, and hyphens without spaces.', 'warning');
		return;
	}

	const formBox = document.getElementById('scaffold-form-content');
	const loadingBox = document.getElementById('scaffold-loading-state');
	if (formBox) formBox.style.display = 'none';
	if (loadingBox) loadingBox.style.display = 'block';

	try {
		const res = await fetch('/api/projects/scaffold', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				name, stack, port, collation, php_version, node_version, go_version
			})
		});
		const data = await res.json();
		if (res.ok) {
			closeScaffoldModal();
			if (typeof showToast === 'function') showToast('Project Created', `Successfully scaffolded ${stack} project inside ${name}!`, 'success');
			loadProjects();
		} else {
			if (formBox) formBox.style.display = 'block';
			if (loadingBox) loadingBox.style.display = 'none';
			if (typeof showToast === 'function') showToast('Creation Failed', data.error || data || 'Could not create project.', 'danger');
		}
	} catch (e) {
		if (formBox) formBox.style.display = 'block';
		if (loadingBox) loadingBox.style.display = 'none';
		if (typeof showToast === 'function') showToast('Network Error', e.message, 'danger');
	}
}
