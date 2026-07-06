function formatRuntimeVersion(key) {
	if (!key || key === 'None') return '';
	if (key.startsWith('php-')) return key.replace('php-', 'PHP ');
	if (key.startsWith('node-')) return key.replace('node-', 'Node ');
	if (key.startsWith('go-')) return key.replace('go-', 'Go ');
	return key;
}

async function setWorkspaceRoot() {
	const pathEl = document.getElementById('workspace-input');
	const btn = document.getElementById('btn-browse-folder');

	if (btn) {
		btn.innerHTML = `<span class="material-symbols-outlined" style="animation:spin 1s linear infinite;">progress_activity</span> Waiting...`;
		btn.disabled = true;
	}

	try {
		const res = await fetch('/api/system/select-folder');
		const data = await res.json();

		if (data.status === 'cancelled' || !data.path) {
			return; // User cancelled
		}

		if (pathEl) {
			pathEl.value = data.path;
		}

		if (typeof showToast === 'function') showToast('Scanning Workspace...', `Scanning applications in ${data.path}`, 'info');

		await fetch('/api/workspace/set', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ workspace: data.path })
		});

		if (typeof showToast === 'function') showToast('Workspace Root Updated', `Successfully mapped to ${data.path}`, 'success');
		loadProjects();
	} catch (e) {
		if (typeof showToast === 'function') showToast('Error', e.message, 'danger');
	} finally {
		if (btn) {
			btn.innerHTML = `<span class="material-symbols-outlined" style="font-size:1.2rem;">folder_open</span> Browse Folder`;
			btn.disabled = false;
		}
	}
}

function getDeviconHTML(stack, framework) {
	const fw = (framework || stack || '').toLowerCase();
	const st = (stack || '').toLowerCase();
	if (fw === 'laravel' || st === 'laravel') {
		return `<i class="devicon-laravel-plain colored" style="font-size:1.6rem;"></i>`;
	}
	if (fw === 'next.js' || fw === 'nextjs') {
		return `<i class="devicon-nextjs-original" style="font-size:1.6rem; color:#fff;"></i>`;
	}
	if (fw === 'vue.js' || fw === 'vue') {
		return `<i class="devicon-vuejs-plain colored" style="font-size:1.6rem;"></i>`;
	}
	if (fw === 'react') {
		return `<i class="devicon-react-original colored" style="font-size:1.6rem;"></i>`;
	}
	if (fw === 'go' || st === 'go') {
		return `<i class="devicon-go-plain colored" style="font-size:1.6rem; color:#38bdf8;"></i>`;
	}
	if (fw === 'fastapi') {
		return `<i class="devicon-fastapi-plain colored" style="font-size:1.6rem; color:#059669;"></i>`;
	}
	if (fw === 'django') {
		return `<i class="devicon-django-plain colored" style="font-size:1.6rem; color:#092e20;"></i>`;
	}
	if (fw === 'nestjs') {
		return `<i class="devicon-nestjs-plain colored" style="font-size:1.6rem; color:#e0234e;"></i>`;
	}
	if (fw === 'nuxt') {
		return `<i class="devicon-nuxtjs-plain colored" style="font-size:1.6rem; color:#00dc82;"></i>`;
	}
	if (st === 'node' || fw === 'node.js') {
		return `<i class="devicon-nodejs-plain colored" style="font-size:1.6rem;"></i>`;
	}
	if (st === 'php' || fw === 'simple php') {
		return `<i class="devicon-php-plain colored" style="font-size:1.6rem; color:#818cf8;"></i>`;
	}
	return `<span class="material-symbols-outlined" style="font-size:1.5rem; color:#60a5fa;">folder</span>`;
}

function escapePath(path) {
	return path.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
}

/* ==============================================================================
 * 🎨 RESTRICTED CONFIGURATIONS (UI STYLES & BADGES)
 * ============================================================================== */
const stackBadgeStyles = {
	'laravel': { bg: 'rgba(255,45,32,0.15)', color: '#ff2d20', border: 'rgba(255,45,32,0.35)' },
	'simple-php': { bg: 'rgba(97,78,166,0.15)', color: '#9b7aff', border: 'rgba(97,78,166,0.35)' },
	'php': { bg: 'rgba(97,78,166,0.15)', color: '#9b7aff', border: 'rgba(97,78,166,0.35)' },
	'go': { bg: 'rgba(0,173,216,0.15)', color: '#00add8', border: 'rgba(0,173,216,0.35)' },
	'nextjs': { bg: 'rgba(255,255,255,0.10)', color: '#ffffff', border: 'rgba(255,255,255,0.25)' },
	'vue': { bg: 'rgba(0,220,130,0.15)', color: '#00dc82', border: 'rgba(0,220,130,0.35)' },
	'fastapi': { bg: 'rgba(5,150,105,0.15)', color: '#10b981', border: 'rgba(5,150,105,0.35)' },
	'node': { bg: 'rgba(102,194,102,0.15)', color: '#68c15a', border: 'rgba(102,194,102,0.35)' },
};
/* ============================================================================== */

function getStackBadge(stack, framework, iconHtml) {
	const s = stackBadgeStyles[stack] || { bg: 'rgba(96,165,250,0.15)', color: '#60a5fa', border: 'rgba(96,165,250,0.3)' };
	return `<span class="badge" style="background:${s.bg}; color:${s.color}; border:1px solid ${s.border}; font-size:0.7rem; font-weight:700; display:inline-flex; align-items:center; gap:4px;">${iconHtml} ${framework}</span>`;
}

function getStudioOpener(path, stack, phpVer, domain, nodeVer, goVer, status) {
	return ''; // Obsolete
}

function getStudioLabel(stack) {
	return 'Extensions';
}

const PHPStacks = new Set(['laravel', 'simple-php', 'php']);

let loadProjectsRetryCount = 0;
let loadProjectsRetryTimer = null;

function setButtonLoading(btn, loading, text) {
	if (!btn) return;
	if (loading) {
		btn.dataset.origText = btn.innerHTML;
		btn.disabled = true;
		btn.style.opacity = '0.6';
		btn.style.pointerEvents = 'none';
		btn.innerHTML = `<span class="material-symbols-outlined" style="animation:spin 1s linear infinite;">progress_activity</span> ${text || 'Loading...'}`;
	} else {
		btn.disabled = false;
		btn.style.opacity = '1';
		btn.style.pointerEvents = 'auto';
		if (btn.dataset.origText) btn.innerHTML = btn.dataset.origText;
	}
}

async function loadProjects(isRetry = false) {
	const container = document.getElementById('projects-cards-container');
	if (!container) return;

	if (!isRetry) {
		loadProjectsRetryCount = 0;
		if (loadProjectsRetryTimer) clearTimeout(loadProjectsRetryTimer);
	}

	try {
		const wsRes = await fetch('/api/workspace/get');
		const wsData = await wsRes.json();
		const inp = document.getElementById('workspace-input');
		const card = document.getElementById('workspace-card');
		if (inp && wsData) {
			if (wsData.workspace) {
				if (document.activeElement !== inp) inp.value = wsData.workspace;
				if (card) {
					card.style.border = '';
					card.style.boxShadow = '';
					const titleEl = card.querySelector('h2');
					if (titleEl) titleEl.innerHTML = 'Workspace Root Directory';
				}
			} else {
				if (card) {
					card.style.border = '1px solid rgba(239, 68, 68, 0.6)';
					card.style.boxShadow = '0 0 15px rgba(239, 68, 68, 0.2)';
					const titleEl = card.querySelector('h2');
					if (titleEl) titleEl.innerHTML = 'Workspace Root Directory <span style="color:#f87171; font-size:0.85rem; margin-left:10px; font-weight:normal;">(Please specify your workspace!)</span>';
				}
			}
		}
	} catch (e) { }

	try {
		const res = await fetch('/api/projects/list-full');
		if (!res.ok) throw new Error('HTTP ' + res.status);
		const projects = await res.json();
		loadProjectsRetryCount = 0;
		if (!projects || projects.length === 0) {
			container.innerHTML = `<div class="card" style="text-align:center; padding:50px 20px;">
				<span class="material-symbols-outlined" style="font-size:3.2rem; color:rgba(255,255,255,0.2); margin-bottom:14px; display:block;">folder_off</span>
				<h3 style="color:#fff; margin-bottom:8px; font-size:1.2rem;">No Applications Detected</h3>
				<p style="color:rgba(255,255,255,0.4); font-size:0.9rem;">Click "+ Create New Project" above to scaffold your first starter project.</p>
			</div>`;
			return;
		}

		const cards = projects.map((p) => {
			const uuid = p.uuid;
			const cleanPath = escapePath(p.path);
			const rawPort = p.port || 8001;
			const activePort = p.serve_port || rawPort;
			const phpVer = p.php_version || '';
			const nodeVer = p.node_version || '';
			const goVer = p.go_version || '';
			const dbName = p.db_name || 'app_' + (p.name || '').toLowerCase().replace(/[^a-z0-9_]/g, '_');
			const stack = (p.stack || 'laravel').toLowerCase();
			const framework = p.framework || (stack === 'go' ? 'Go' : 'Laravel');
			const iconHtml = getDeviconHTML(stack, framework);
			const stackBadge = getStackBadge(stack, framework, iconHtml);
			const isConfigured = (p.status === 'Configured' || p.status === 'Desynced');
			const isRunning = p.serve_running || false;
			const isWorker = p.queue_running || false;
			const domain = `http://127.0.0.1:${activePort}`;
			const studioOpener = getStudioOpener(p.path, stack, phpVer, domain, p.node_version, p.go_version, p.status);
			const studioLabel = getStudioLabel(stack);
			const showQueue = PHPStacks.has(stack);

			if (!isConfigured) {
				const expectedDb = 'app_' + p.name.replace(/[^a-zA-Z0-9_]/g, '_').toLowerCase();
				const showPhp = PHPStacks.has(stack);
				const showNode = (stack === 'laravel' || stack === 'node' || stack === 'nextjs' || stack === 'vue');
				const showGo = (stack === 'go');
				const showDb = showPhp || showNode;

				return `
				<div class="project-panel" style="border-color:rgba(245,158,11,0.3); background:linear-gradient(160deg,rgba(40,30,15,0.65) 0%,rgba(20,16,8,0.5) 100%);"><div class="panel-top" style="border-bottom-color:rgba(245,158,11,0.15);"><div style="display:flex; align-items:center; gap:14px;"><div style="width:44px; height:44px; border-radius:12px; background:rgba(245,158,11,0.15); border:1px solid rgba(245,158,11,0.3); display:flex; align-items:center; justify-content:center; flex-shrink:0;">${iconHtml}</div><div><div style="display:flex; align-items:center; gap:10px; margin-bottom:4px; flex-wrap:wrap;"><h3 style="margin:0; font-size:1.05rem; font-weight:700; color:#f8fafc;">${p.name}</h3>${stackBadge}<span class="badge" style="background:rgba(245,158,11,0.15); color:#f59e0b; border:1px solid rgba(245,158,11,0.3); font-size:0.72rem;"><span class="material-symbols-outlined" style="font-size:0.9em;">warning</span> Pending Configuration</span>${showDb ? `<span class="badge" style="background:rgba(16,185,129,0.15); color:#34d399; border:1px solid rgba(16,185,129,0.3); font-size:0.72rem;">DB: ${expectedDb}</span>` : ''}</div><div style="font-size:0.78rem; color:rgba(255,255,255,0.35); font-family:'JetBrains Mono';">${p.path}</div></div></div><div style="display:flex; flex-direction:column; align-items:flex-end; gap:10px;">
				<div style="display:flex; gap:8px; align-items:center; flex-wrap:wrap;">
				<select id="php-ver-${uuid}" class="select-mini" style="width:110px;" title="Select PHP Version"><option value="php-8.5" ${phpVer === 'php-8.5' ? 'selected' : ''}>PHP 8.5</option><option value="php-8.4" ${phpVer === 'php-8.4' ? 'selected' : ''}>PHP 8.4</option><option value="php-8.3" ${phpVer === 'php-8.3' ? 'selected' : ''}>PHP 8.3</option><option value="php-8.2" ${phpVer === 'php-8.2' ? 'selected' : ''}>PHP 8.2</option><option value="" ${!phpVer ? 'selected' : ''}>None</option></select>
				<select id="node-ver-${uuid}" class="select-mini" style="width:110px;" title="Select Node.js Version"><option value="node-v24" ${nodeVer === 'node-v24' ? 'selected' : ''}>Node v24</option><option value="node-v22" ${nodeVer === 'node-v22' ? 'selected' : ''}>Node v22</option><option value="node-v20" ${nodeVer === 'node-v20' ? 'selected' : ''}>Node v20</option><option value="" ${!nodeVer ? 'selected' : ''}>None</option></select>
				<select id="go-ver-${uuid}" class="select-mini" style="width:110px;" title="Select Go Version"><option value="go-1.26.4" ${goVer === 'go-1.26.4' ? 'selected' : ''}>Go 1.26.4</option><option value="go-1.23.4" ${goVer === 'go-1.23.4' ? 'selected' : ''}>Go 1.23.4</option><option value="" ${!goVer ? 'selected' : ''}>None</option></select>
				${showDb ? `<select id="collation-${uuid}" class="select-mini" style="width:160px;" title="Database Collation"><option value="utf8mb4_unicode_ci">utf8mb4_unicode_ci</option><option value="utf8mb4_general_ci">utf8mb4_general_ci</option><option value="utf8mb4_persian_ci">utf8mb4_persian_ci</option></select>` : ''}
				<input type="number" id="port-${uuid}" class="select-mini" style="width:75px; text-align:center;" value="${rawPort}" placeholder="Port">
				<button class="btn btn-warning" onclick="configureProject('${cleanPath}', '${uuid}', this)" style="padding:9px 20px;"><span class="material-symbols-outlined">build</span> Configure Application</button>
				</div><div style="font-size:0.74rem; color:rgba(245,158,11,0.55);">Select runtimes, port, collation before configuring.</div>
				</div></div></div>`;
			}

			return `
			<div class="project-panel" style="${isRunning ? 'border-color:rgba(52,211,153,0.3);' : ''}">
				<div class="panel-top">
					<div style="display:flex; align-items:center; gap:14px;">
						<div style="width:46px; height:46px; border-radius:13px; background:linear-gradient(135deg,rgba(37,99,235,0.2),rgba(29,78,216,0.3)); border:1px solid rgba(147,197,253,0.25); display:flex; align-items:center; justify-content:center; flex-shrink:0; box-shadow:0 4px 14px rgba(37,99,235,0.2);">${iconHtml}</div>
						<div>
							<div style="display:flex; align-items:center; gap:10px; margin-bottom:4px; flex-wrap:wrap;">
								<span style="font-size:1.08rem; font-weight:700; color:#fff;">${p.name}</span>
								${stackBadge}
								${p.status === 'Desynced' ? '<span class="badge" style="background:rgba(239,68,68,0.15); color:#f87171; border:1px solid rgba(239,68,68,0.3); font-size:0.7rem;"><span class="material-symbols-outlined" style="font-size:0.9em;">warning</span> Not Synced</span>' : '<span class="badge badge-active" style="font-size:0.7rem;"><span class="material-symbols-outlined" style="font-size:0.9em;">check_circle</span> Configured</span>'}
								${isRunning ? `<span class="badge" style="background:rgba(52,211,153,0.12); color:#34d399; border:1px solid rgba(52,211,153,0.3); font-size:0.7rem;"><span class="material-symbols-outlined" style="font-size:0.9em;">radio_button_checked</span> Live :${activePort}</span>` : ''}
								${isWorker ? `<span class="badge" style="background:rgba(168,85,247,0.12); color:#c084fc; border:1px solid rgba(168,85,247,0.3); font-size:0.7rem;"><span class="material-symbols-outlined" style="font-size:0.9em;">precision_manufacturing</span> Worker</span>` : ''}
								${phpVer ? `<span class="badge" style="background:rgba(139,92,246,0.12); color:#a78bfa; border:1px solid rgba(139,92,246,0.3); font-size:0.68rem;"><span class="material-symbols-outlined" style="font-size:0.9em;">code</span> ${formatRuntimeVersion(phpVer)}</span>` : ''}
								${nodeVer ? `<span class="badge" style="background:rgba(34,197,94,0.1); color:#86efac; border:1px solid rgba(34,197,94,0.25); font-size:0.68rem;"><span class="material-symbols-outlined" style="font-size:0.9em;">hub</span> ${formatRuntimeVersion(nodeVer)}</span>` : ''}
								${goVer ? `<span class="badge" style="background:rgba(56,189,248,0.1); color:#7dd3fc; border:1px solid rgba(56,189,248,0.25); font-size:0.68rem;"><span class="material-symbols-outlined" style="font-size:0.9em;">terminal</span> ${formatRuntimeVersion(goVer)}</span>` : ''}
							</div>
							<div style="display:flex; align-items:center; gap:12px;">
								<span style="font-size:0.77rem; color:rgba(255,255,255,0.35); font-family:'JetBrains Mono';">${p.path}</span>
								${dbName ? `<span style="font-size:0.75rem; color:rgba(255,255,255,0.3); display:flex; align-items:center; gap:4px;"><span class="material-symbols-outlined" style="font-size:0.95em;">database</span>${dbName}</span>` : ''}
							</div>
						</div>
					</div>
					<div style="display:flex; gap:8px; align-items:center; flex-wrap:wrap;">
						${isRunning ? `
							<a href="${domain}" target="_blank" class="btn btn-outline" style="border-color:rgba(52,211,153,0.5); color:#34d399; text-decoration:none;"><span class="material-symbols-outlined">public</span> Open Site</a>
							<button class="btn btn-outline" style="border-color:rgba(239,68,68,0.45); color:#f87171;" onclick="stopServe('${cleanPath}', this)"><span class="material-symbols-outlined">stop_circle</span> Stop</button>
						` : `
							<button class="btn btn-primary" onclick="startServe('${cleanPath}', '${uuid}', this)"><span class="material-symbols-outlined">play_arrow</span> Start Server</button>
						`}
						${showQueue ? (isWorker ? `
							<button class="btn btn-outline" style="border-color:rgba(239,68,68,0.45); color:#f87171;" onclick="stopQueue('${cleanPath}', this)"><span class="material-symbols-outlined">stop_circle</span> Worker</button>
						` : `
							<button class="btn btn-blue" onclick="startQueue('${cleanPath}', '${uuid}', this)"><span class="material-symbols-outlined">precision_manufacturing</span> Queue</button>
						`) : ''}
					</div>
				</div>
				<div class="panel-bot" style="justify-content:space-between;">
					<div style="display:flex; align-items:center; gap:10px; flex-wrap:wrap;">
						<span style="font-size:0.78rem; font-weight:600; color:rgba(255,255,255,0.35); text-transform:uppercase;">PHP</span>
						<select id="php-ver-${uuid}" class="select-mini" style="width:105px;"><option value="php-8.5" ${phpVer === 'php-8.5' ? 'selected' : ''}>PHP 8.5</option><option value="php-8.4" ${phpVer === 'php-8.4' ? 'selected' : ''}>PHP 8.4</option><option value="php-8.3" ${phpVer === 'php-8.3' ? 'selected' : ''}>PHP 8.3</option><option value="php-8.2" ${phpVer === 'php-8.2' ? 'selected' : ''}>PHP 8.2</option><option value="" ${!phpVer ? 'selected' : ''}>None</option></select>
						<span style="font-size:0.78rem; font-weight:600; color:rgba(255,255,255,0.35); text-transform:uppercase;">Node</span>
						<select id="node-ver-${uuid}" class="select-mini" style="width:105px;"><option value="node-v24" ${nodeVer === 'node-v24' ? 'selected' : ''}>Node v24</option><option value="node-v22" ${nodeVer === 'node-v22' ? 'selected' : ''}>Node v22</option><option value="node-v20" ${nodeVer === 'node-v20' ? 'selected' : ''}>Node v20</option><option value="" ${!nodeVer ? 'selected' : ''}>None</option></select>
						<span style="font-size:0.78rem; font-weight:600; color:rgba(255,255,255,0.35); text-transform:uppercase;">Go</span>
						<select id="go-ver-${uuid}" class="select-mini" style="width:105px;"><option value="go-1.26.4" ${goVer === 'go-1.26.4' ? 'selected' : ''}>Go 1.26.4</option><option value="go-1.23.4" ${goVer === 'go-1.23.4' ? 'selected' : ''}>Go 1.23.4</option><option value="" ${!goVer ? 'selected' : ''}>None</option></select>
						<span style="font-size:0.78rem; font-weight:600; color:rgba(255,255,255,0.35); text-transform:uppercase;">Port</span>
						<input type="number" id="port-${uuid}" class="select-mini" style="width:72px; text-align:center;" value="${rawPort}">
						<button class="btn btn-outline" style="padding:6px 13px; font-size:0.8rem;" onclick="configureProject('${cleanPath}', '${uuid}', this)"><span class="material-symbols-outlined" style="font-size:1.05em;">sync</span> Re-Sync</button>
					</div>
					<div style="display:flex; align-items:center; gap:6px;">
						<button class="btn-mini-action vscode" title="Open in VS Code" onclick="fetch('/api/project/code?path=' + encodeURIComponent('${cleanPath}')).then(r=>r.json()).then(d=>{ if(d.error && typeof showToast==='function') showToast('VS Code Error', d.error, 'danger'); })"><span class="material-symbols-outlined" style="font-size:1.1rem;">code</span></button>
						<button class="btn-mini-action folder" title="Open Folder" onclick="fetch('/api/project/open?path=' + encodeURIComponent('${cleanPath}'))"><span class="material-symbols-outlined" style="font-size:1.1rem;">folder</span></button>
						<button class="btn-mini-action terminal" title="Launch Terminal" onclick="fetch('/api/projects/terminal', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({path:'${cleanPath}'})})" ><span class="material-symbols-outlined" style="font-size:1.1rem;">terminal</span></button>
						<button class="btn-mini-action eject" title="Eject Project" onclick="ejectProject('${cleanPath}', '${p.db_name || ''}', '${p.uuid || ''}')"><span class="material-symbols-outlined" style="font-size:1.1rem;">eject</span></button>
						<button class="btn-mini-action delete" title="Remove from list" onclick="deleteProject('${cleanPath}', '${p.db_name || ''}', '${p.uuid || ''}')"><span class="material-symbols-outlined" style="font-size:1.1rem;">delete</span></button>
					</div>
				</div>
			</div>`;
		});
		container.innerHTML = cards.join('');

		// Apply spotlight effect on mouse move
		document.querySelectorAll('.project-panel').forEach(panel => {
			panel.addEventListener('mousemove', e => {
				const rect = panel.getBoundingClientRect();
				const x = e.clientX - rect.left;
				const y = e.clientY - rect.top;
				panel.style.setProperty('--x', `${x}px`);
				panel.style.setProperty('--y', `${y}px`);
			});
		});
	} catch (e) {
		if (loadProjectsRetryCount < 3) {
			loadProjectsRetryCount++;
			const delay = Math.pow(2, loadProjectsRetryCount) * 1000;
			console.warn(`[Silent Retry] Connection lost. Retrying loadProjects in ${delay}ms (Attempt ${loadProjectsRetryCount}/3)...`);
			loadProjectsRetryTimer = setTimeout(() => loadProjects(true), delay);
			if (typeof showToast === 'function' && loadProjectsRetryCount === 1) {
				showToast('Reconnecting...', 'Connection to Bridge API interrupted. Reconnecting automatically...', 'warning');
			}
			return;
		}
		container.innerHTML = `<div class="card" style="text-align:center; color:#f87171; padding:40px;">
			<span class="material-symbols-outlined" style="font-size:2.8rem; margin-bottom:12px; display:block;">error</span>
			Failed to connect to Bridge Engine API. Ensure bridge.exe is active.
		</div>`;
	}
}

async function scanWorkspaceProjects() {
	if (typeof showToast === 'function') showToast('Scanning Workspace', 'Running discovery engine to detect new projects...', 'info');
	try {
		const res = await fetch('/api/projects/scan');
		const data = await res.json();
		if (data.error === 'workspace_not_set') {
			if (typeof showToast === 'function') showToast('Warning', 'Please specify a workspace directory first!', 'warning');
			return;
		}
		loadProjects();
	} catch (e) {
		console.error(e);
	}
}

function deleteProject(path, dbName, uuid) {
	showConfirmModal(
		"Hard Delete Project",
		"⚠️ <strong>CRITICAL WARNING:</strong> Are you sure you want to completely delete this project and its data?<br><br>• All project data and its dedicated database will be wiped from the system.<br>• The project folder and all files will be permanently deleted from the hard drive (Hard Delete).<br><br>💡 <em>Recommendation: Please take a backup if you haven't already.</em>",
		async () => {
			if (typeof showToast === 'function') showToast("Deleting Project...", "Removing database and deleting project directory from disk...", "warning");
			try {
				const res = await fetch('/api/projects/delete', {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ path, db_name: dbName || '', uuid: uuid || '' })
				});
				if (res.ok) {
					if (typeof showToast === 'function') showToast("Project Deleted", "Project directory and database deleted successfully.", "success");
				} else {
					const d = await res.json().catch(() => ({}));
					if (typeof showToast === 'function') showToast("Delete Failed", d.error || "Failed to delete project.", "danger");
				}
			} catch (e) {
				if (typeof showToast === 'function') showToast("Delete Error", e.message, "danger");
			} finally {
			}
		}
	);
}

function ejectProject(path, dbName, uuid) {
	showConfirmModal(
		"Eject Project",
		"⚠️ <strong>WARNING:</strong> This will remove the project from the panel and delete its configuration files.<br><br>• The <code>.env</code> file will be restored to its original state.<br>• The dedicated database for this project will be dropped.<br>• Your original project files on the hard drive will remain untouched.",
		async () => {
			if (typeof showToast === 'function') showToast("Ejecting Project...", "Reverting configuration and removing database...", "warning");
			try {
				const res = await fetch('/api/projects/eject', {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ path, db_name: dbName || '', uuid: uuid || '' })
				});
				if (res.ok) {
					if (typeof showToast === 'function') showToast("Project Ejected", "Project successfully reverted to unconfigured state.", "success");
				} else {
					const d = await res.json().catch(() => ({}));
					if (typeof showToast === 'function') showToast("Eject Failed", d.error || "Failed to eject project.", "danger");
				}
			} catch (e) {
				if (typeof showToast === 'function') showToast("Eject Error", e.message, "danger");
			} finally {
				scanWorkspaceProjects();
			}
		}
	);
}

async function configureProject(path, uuid, btn) {
	setButtonLoading(btn, true, 'Syncing...');
	const phpVer = document.getElementById(`php-ver-${uuid}`)?.value ?? '';
	const nodeVer = document.getElementById(`node-ver-${uuid}`)?.value ?? '';
	const goVer = document.getElementById(`go-ver-${uuid}`)?.value ?? '';
	const port = document.getElementById(`port-${uuid}`)?.value || '8000';
	const collation = document.getElementById(`collation-${uuid}`)?.value || 'utf8mb4_unicode_ci';
	try {
		await fetch('/api/projects/provision', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ path, php_version: phpVer, node_version: nodeVer, go_version: goVer, port, collation })
		});
		if (typeof showToast === 'function') showToast('Configuration Complete', 'Application configured successfully (.env injected, database ready, local shims compiled).', 'success');
	} finally {
		setButtonLoading(btn, false);
		loadProjects();
	}
}

async function startServe(path, uuid, btn) {
	setButtonLoading(btn, true, 'Starting...');
	const port = document.getElementById(`port-${uuid}`).value;
	const phpVer = document.getElementById(`php-ver-${uuid}`)?.value ?? '';
	const nodeVer = document.getElementById(`node-ver-${uuid}`)?.value ?? '';
	const goVer = document.getElementById(`go-ver-${uuid}`)?.value ?? '';
	try {
		await fetch('/api/serve/start', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path, port, php_version: phpVer, node_version: nodeVer, go_version: goVer }) });
	} finally {
		setButtonLoading(btn, false);
		setTimeout(() => loadProjects(), 400);
	}
}

async function stopServe(path, btn) {
	setButtonLoading(btn, true, 'Stopping...');
	try {
		await fetch('/api/serve/stop', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path }) });
	} finally {
		setButtonLoading(btn, false);
		setTimeout(() => loadProjects(), 400);
	}
}

async function startQueue(path, uuid, btn) {
	setButtonLoading(btn, true, 'Queue...');
	const phpVer = document.getElementById(`php-ver-${uuid}`)?.value || '';
	try {
		await fetch('/api/queue/start', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path, php_version: phpVer }) });
	} finally {
		setButtonLoading(btn, false);
		setTimeout(() => loadProjects(), 400);
	}
}

async function stopQueue(path, btn) {
	setButtonLoading(btn, true, 'Stopping...');
	try {
		await fetch('/api/queue/stop', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path }) });
	} finally {
		setButtonLoading(btn, false);
		setTimeout(() => loadProjects(), 400);
	}
}


