function toggleDock() {
	const dock = document.getElementById('terminal-dock');
	const btn = document.getElementById('dock-toggle-btn');
	if (dock) {
		dock.classList.toggle('collapsed');
		if (btn) btn.innerHTML = dock.classList.contains('collapsed') 
			? '<span class="material-symbols-outlined">keyboard_arrow_up</span> Expand Dock' 
			: '<span class="material-symbols-outlined">keyboard_arrow_down</span> Collapse Dock';
	}
}

async function checkCommandLogs() {
	try {
		const res = await fetch('/api/system/console/logs');
		const text = await res.text();
		if (text && text.trim().length > 0) {
			const dockBody = document.getElementById('dock-logs-output');
			if (dockBody) {
				const formatted = text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/\n/g, '<br>');
				if (dockBody.dataset.lastLog !== formatted) {
					dockBody.innerHTML = formatted;
					dockBody.dataset.lastLog = formatted;
					dockBody.scrollTop = dockBody.scrollHeight;
				}
			}
		}
	} catch (e) {}
}

async function clearLogs() {
	const dockBody = document.getElementById('dock-logs-output');
	if (dockBody) {
		dockBody.innerHTML = '[Ready] Console logs cleared.';
		dockBody.dataset.lastLog = '';
	}
	await fetch('/api/system/console/clear', { method: 'POST' });
}

let activeCommandPath = '';
async function runCommand(path, index) {
	const cmdType = document.getElementById(`cmd-sel-${index}`).value;
	const dock = document.getElementById('terminal-dock');
	const dockBody = document.getElementById('dock-logs-output');
	if (dock && dock.classList.contains('collapsed')) toggleDock();
	if (dockBody) dockBody.innerHTML = `Executing [${cmdType}] on ${path}...\n(Live output streaming from Bridge Engine...)`;
	activeCommandPath = path;
	await fetch('/api/command/run', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path, command_type: cmdType }) });
}

async function openTerminal(path) {
	await fetch('/api/projects/terminal', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path }) });
}
