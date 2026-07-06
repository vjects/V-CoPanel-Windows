async function loadComponent(id, url) {
	let el = document.getElementById(id);
	if (!el) {
		if (id.startsWith('view-')) {
			const contentArea = document.querySelector('.content-area');
			if (contentArea) {
				el = document.createElement('div');
				el.id = id;
				contentArea.appendChild(el);
			} else return;
		} else return;
	}
	try {
		const cacheBuster = `?cb=${new Date().getTime()}`;
		const res = await fetch(url.includes('?') ? url + '&cb=' + new Date().getTime() : url + cacheBuster);
		el.innerHTML = await res.text();
	} catch (e) {
		console.error(`Failed to load component ${url}:`, e);
	}
}

window.TabManager = {
	activeTab: 'tab-projects',
	mountHooks: {},
	unmountHooks: {},
	intervals: {},

	onMount: function(tabId, callback) {
		if (!this.mountHooks[tabId]) this.mountHooks[tabId] = [];
		this.mountHooks[tabId].push(callback);
	},
	onUnmount: function(tabId, callback) {
		if (!this.unmountHooks[tabId]) this.unmountHooks[tabId] = [];
		this.unmountHooks[tabId].push(callback);
	},
	registerInterval: function(tabId, intervalId) {
		if (!this.intervals[tabId]) this.intervals[tabId] = [];
		this.intervals[tabId].push(intervalId);
	},
	switch: function(newTabId) {
		const oldTabId = this.activeTab;
		if (oldTabId && oldTabId !== newTabId) {
			if (this.intervals[oldTabId]) {
				this.intervals[oldTabId].forEach(id => clearInterval(id));
				this.intervals[oldTabId] = [];
			}
			if (this.unmountHooks[oldTabId]) {
				this.unmountHooks[oldTabId].forEach(cb => { try { cb(); } catch(e){} });
			}
		}
		this.activeTab = newTabId;
		if (this.mountHooks[newTabId]) {
			this.mountHooks[newTabId].forEach(cb => { try { cb(); } catch(e){} });
		}
	}
};

window.StudioManager = {
	setupDirtyState: function(syncBtnSelector, serveBtnSelector, inputSelectors) {
		let syncBtn = typeof syncBtnSelector === 'string' ? document.querySelector(syncBtnSelector) : syncBtnSelector;
		let serveBtn = typeof serveBtnSelector === 'string' ? document.querySelector(serveBtnSelector) : serveBtnSelector;
		if (!syncBtn) return;

		inputSelectors.forEach(sel => {
			let el = typeof sel === 'string' ? document.querySelector(sel) : sel;
			if (!el) return;
			el.addEventListener('change', () => {
				syncBtn.style.borderColor = '#f59e0b';
				syncBtn.style.color = '#f59e0b';
				syncBtn.style.boxShadow = '0 0 14px rgba(245,158,11,0.6)';
				syncBtn.style.animation = 'pulse 1.5s infinite';
				if (!syncBtn.dataset.origText) syncBtn.dataset.origText = syncBtn.innerHTML;
				syncBtn.innerHTML = `<span class="material-symbols-outlined" style="animation:spin 1s linear infinite;">sync_problem</span> Sync Required`;
				
				if (serveBtn) {
					serveBtn.disabled = true;
					serveBtn.style.opacity = '0.5';
					serveBtn.title = 'Please click Sync Shims before starting server!';
				}
			});
		});
	},
	clearDirtyState: function(syncBtnSelector, serveBtnSelector) {
		let syncBtn = typeof syncBtnSelector === 'string' ? document.querySelector(syncBtnSelector) : syncBtnSelector;
		let serveBtn = typeof serveBtnSelector === 'string' ? document.querySelector(serveBtnSelector) : serveBtnSelector;
		if (syncBtn) {
			syncBtn.style.borderColor = '';
			syncBtn.style.color = '';
			syncBtn.style.boxShadow = '';
			syncBtn.style.animation = '';
			if (syncBtn.dataset.origText) syncBtn.innerHTML = syncBtn.dataset.origText;
		}
		if (serveBtn) {
			serveBtn.disabled = false;
			serveBtn.style.opacity = '1';
			serveBtn.title = '';
		}
	},
	toggleAuxMenu: function(btn, menuId) {
		const menu = document.getElementById(menuId);
		if (!menu) return;
		if (menu.style.display === 'none' || !menu.style.display) {
			menu.style.display = 'flex';
			if (btn) btn.innerHTML = `<span class="material-symbols-outlined">expand_less</span> Hide Extensions`;
		} else {
			menu.style.display = 'none';
			if (btn) btn.innerHTML = `<span class="material-symbols-outlined">extension</span> Auxiliary Extensions (+)`;
		}
	}
};

const origFetch = window.fetch;
window.fetch = async function(...args) {
	const url = typeof args[0] === 'string' ? args[0] : (args[0] && args[0].url ? args[0].url : '');
	const res = await origFetch.apply(this, args);
	if (url.includes('/api/projects/provision') && res.ok) {
		const activeTabEl = document.querySelector('.tab-content.active');
		if (activeTabEl) {
			const syncBtn = activeTabEl.querySelector('[onclick*="rovision"], [onclick*="ync"]');
			const serveBtn = activeTabEl.querySelector('[onclick*="tartServe"], [onclick*="erve"]');
			if (window.StudioManager) window.StudioManager.clearDirtyState(syncBtn, serveBtn);
		}
	}
	return res;
};

function initSSE() {
	if (!window.EventSource) return;
	const source = new EventSource('/api/events/stream');
	source.addEventListener('telemetry', () => {
		try {
			if (typeof loadSystemStats === 'function') loadSystemStats();
			if (typeof checkMailpitStatus === 'function') checkMailpitStatus();
			if (typeof checkPMAStatus === 'function') checkPMAStatus();
			if (typeof loadEnginePrecheck === 'function') loadEnginePrecheck(true);
			if (typeof loadSidebarContext === 'function') loadSidebarContext();
			if (typeof loadTopbarContext === 'function') loadTopbarContext();
		} catch (e) {}
	});
}

function switchTab(tabId) {
	document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
	document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
	
	const targetTab = document.getElementById(tabId);
	if (targetTab) targetTab.classList.add('active');
	
	const clickedNavItem = document.querySelector(`[onclick="switchTab('${tabId}')"]`);
	if (clickedNavItem) clickedNavItem.classList.add('active');
	
	if (window.TabManager) window.TabManager.switch(tabId);
	
	setTimeout(() => {
		const activeTabEl = document.getElementById(tabId);
		if (!activeTabEl) return;
		const syncBtn = activeTabEl.querySelector('[onclick*="rovision"], [onclick*="ync"]');
		const serveBtn = activeTabEl.querySelector('[onclick*="tartServe"], [onclick*="erve"]');
		const selects = activeTabEl.querySelectorAll('select, input[type="number"]');
		
		if (syncBtn && selects.length > 0 && !syncBtn.dataset.dirtyBound) {
			syncBtn.dataset.dirtyBound = "true";
			if (window.StudioManager) window.StudioManager.setupDirtyState(syncBtn, serveBtn, Array.from(selects));
		}
	}, 200);
	
	if (tabId === 'tab-projects') loadProjects();
	if (tabId === 'tab-vcopanel_engine' && typeof loadVCoPanelEngine === 'function') {
		loadVCoPanelEngine();
	}
	if (tabId === 'tab-php_config') {
		if (typeof loadAvailablePHPVersions === 'function') loadAvailablePHPVersions();
		else loadPHPConfig();
	}
	if (tabId === 'tab-node_config') {
		if (typeof loadAvailableNodeVersions === 'function') loadAvailableNodeVersions();
		else if (typeof loadNodeStudio === 'function') loadNodeStudio();
	}
	if (tabId === 'tab-go_config') {
		if (typeof loadAvailableGoVersions === 'function') loadAvailableGoVersions();
		else if (typeof loadGoStudio === 'function') loadGoStudio();
	}
	if (tabId === 'tab-redis_config' && typeof loadRedisConfig === 'function') loadRedisConfig();
	if (tabId === 'tab-mariadb_config' && typeof loadMariaDBConfig === 'function') loadMariaDBConfig();
	if (tabId === 'tab-mailpit_config' && typeof loadMailpitConfig === 'function') loadMailpitConfig();
}

function toggleSidebarCollapse() {
	const sb = document.getElementById('app-sidebar') || document.querySelector('.sidebar');
	if (!sb) return;
	sb.classList.toggle('collapsed');
	const isCol = sb.classList.contains('collapsed');
	localStorage.setItem('sidebar_collapsed', isCol ? '1' : '0');
}

document.addEventListener('DOMContentLoaded', async () => {
	// Load all modular components
	await Promise.all([
		loadComponent('sidebar-placeholder', '/components/sidebar.html'),
		loadComponent('topbar-placeholder', '/components/topbar.html'),
		loadComponent('console-placeholder', '/components/console.html'),
		loadComponent('view-vcopanel_engine-placeholder', '/views/vcopanel_engine.html'),
		loadComponent('view-projects-placeholder', '/views/projects.html'),
		loadComponent('view-php_config-placeholder', '/views/php_config.html'),
		loadComponent('view-node_config-placeholder', '/views/node_config.html'),
		loadComponent('view-go_config-placeholder', '/views/go_config.html'),
		loadComponent('view-redis_config-placeholder', '/views/redis_config.html'),
		loadComponent('view-mariadb_config-placeholder', '/views/mariadb_config.html'),
		loadComponent('view-mailpit_config-placeholder', '/views/mailpit_config.html')
	]);

	if (localStorage.getItem('sidebar_collapsed') === '1') {
		const sb = document.getElementById('app-sidebar') || document.querySelector('.sidebar');
		if (sb) sb.classList.add('collapsed');
	}

	// Initialize data streams
	if (typeof initSystemBoot === 'function') await initSystemBoot();
	await updateDynamicLinks();
	if (typeof loadSidebarContext === 'function') loadSidebarContext();
	if (typeof loadTopbarContext === 'function') loadTopbarContext();
	if (typeof loadProjects === 'function') loadProjects();
	if (typeof loadSystemStats === 'function') loadSystemStats();
	if (typeof checkCommandLogs === 'function') checkCommandLogs();
	if (typeof checkMailpitStatus === 'function') checkMailpitStatus();
	if (typeof checkPMAStatus === 'function') checkPMAStatus();
	if (typeof loadEnginePrecheck === 'function') loadEnginePrecheck(true);

	initSSE();

	setInterval(() => {
		updateDynamicLinks();
		if (typeof loadSidebarContext === 'function') loadSidebarContext();
		if (typeof loadTopbarContext === 'function') loadTopbarContext();
		if (typeof loadSystemStats === 'function') loadSystemStats();
		if (typeof checkMailpitStatus === 'function') checkMailpitStatus();
		if (typeof checkPMAStatus === 'function') checkPMAStatus();
		if (typeof loadEnginePrecheck === 'function') loadEnginePrecheck(true);
	}, 8000);

	setInterval(() => {
		if (typeof checkCommandLogs === 'function') checkCommandLogs();
	}, 1200);
});

async function loadSidebarContext() {
	try {
		const res = await fetch('/api/ui/sidebar-context');
		if (!res.ok) return;
		const data = await res.json();
		const menu = document.getElementById('sidebar-runtimes-menu');
		if (!menu || !data || !data.runtimes) return;
		
		if (data.runtimes.length === 0) {
			menu.innerHTML = `<div style="padding: 8px 14px; font-size: 0.8rem; color: rgba(255,255,255,0.4);">No runtimes installed</div>`;
			return;
		}

		menu.innerHTML = data.runtimes.map(rt => `
			<button class="nav-item" onclick="switchTab('${rt.tab_id}')" title="${rt.title || rt.name}">
				<span class="material-symbols-outlined">${rt.icon || 'code'}</span>
				<span class="nav-label">${rt.name}</span>
			</button>
		`).join('');
	} catch (e) {
		console.error("Failed to load sidebar context:", e);
	}
}

// Topbar Telemetry & Shared Service Monitors
async function loadSystemStats() {
	try {
		const res = await fetch('/api/engine/performance');
		if (!res.ok) return;
		const data = await res.json();
		const elRam = document.getElementById('header-ram-val');
		if (elRam) {
			const osTotal = data.os_total_ram_mb || 16384;
			const osUsed = data.os_used_ram_mb || 4096;
			const pct = Math.round((osUsed / osTotal) * 100);
			elRam.innerText = `${pct}%`;
		}
	} catch (e) {}
}

async function checkMailpitStatus() {
	try {
		const res = await fetch('/api/mailpit/status');
		if (!res.ok) return;
		const data = await res.json();
		const dot = document.getElementById('dot-mailpit');
		const txt = document.getElementById('txt-mailpit');
		if (dot && txt) {
			if (data.running) {
				dot.style.background = '#10b981';
				dot.style.boxShadow = '0 0 8px #10b981';
				txt.style.color = '#10b981';
				const srv = window.SystemServices ? window.SystemServices.find(s => s.service_key === 'mailpit') : null;
				txt.innerText = srv ? srv.port : '8025';
			} else {
				dot.style.background = '#ef4444';
				dot.style.boxShadow = 'none';
				txt.style.color = 'rgba(255,255,255,0.4)';
				txt.innerText = 'Stopped';
			}
		}
	} catch (e) {}
}

async function checkPMAStatus() {
	try {
		const res = await fetch('/api/phpmyadmin/status');
		if (!res.ok) return;
		const data = await res.json();
		
		const dotPma = document.getElementById('dot-pma');
		const txtPma = document.getElementById('txt-pma');
		if (dotPma && txtPma) {
			if (data.running) {
				dotPma.style.background = '#10b981';
				dotPma.style.boxShadow = '0 0 8px #10b981';
				txtPma.style.color = '#10b981';
				const srv = window.SystemServices ? window.SystemServices.find(s => s.service_key === 'phpmyadmin') : null;
				txtPma.innerText = srv ? srv.port : '8881';
			} else {
				dotPma.style.background = '#ef4444';
				dotPma.style.boxShadow = 'none';
				txtPma.style.color = 'rgba(255,255,255,0.4)';
				txtPma.innerText = 'Stopped';
			}
		}

		const dotMaria = document.getElementById('dot-mariadb');
		const txtMaria = document.getElementById('txt-mariadb');
		if (dotMaria && txtMaria) {
			if (data.mariadb_running !== false) {
				dotMaria.style.background = '#10b981';
				dotMaria.style.boxShadow = '0 0 8px #10b981';
				txtMaria.style.color = '#10b981';
				const srv = window.SystemServices ? window.SystemServices.find(s => s.service_key === 'mariadb') : null;
				txtMaria.innerText = srv ? srv.port : '3306';
			} else {
				dotMaria.style.background = '#ef4444';
				dotMaria.style.boxShadow = 'none';
				txtMaria.style.color = 'rgba(255,255,255,0.4)';
				txtMaria.innerText = 'Stopped';
			}
		}

		const resRedis = await fetch('/api/redis/status');
		if (resRedis.ok) {
			const dataRedis = await resRedis.json();
			const dotRedis = document.getElementById('dot-redis');
			const txtRedis = document.getElementById('txt-redis');
			if (dotRedis && txtRedis) {
				if (dataRedis.running) {
					dotRedis.style.background = '#10b981';
					dotRedis.style.boxShadow = '0 0 8px #10b981';
					txtRedis.style.color = '#10b981';
					const srv = window.SystemServices ? window.SystemServices.find(s => s.service_key === 'redis') : null;
					txtRedis.innerText = srv ? srv.port : '6379';
				} else {
					dotRedis.style.background = '#ef4444';
					dotRedis.style.boxShadow = 'none';
					txtRedis.style.color = 'rgba(255,255,255,0.4)';
					txtRedis.innerText = 'Stopped';
				}
			}
		}
	} catch (e) {}
}

async function shutdownSystem() {
	showConfirmModal(
		"Shut Down V-CoPanel",
		"⚠️ Are you sure you want to shut down V-CoPanel Bridge Engine and all active services? This will terminate all running development servers and databases.",
		async () => {
			if (typeof showToast === 'function') {
				showToast("🛑 Shutting Down...", "Stopping all services, databases, and core daemons. Please close this window.");
			}
			try {
				await fetch('/api/system/shutdown', { method: 'POST' });
				setTimeout(() => {
					document.body.innerHTML = `
						<div style="display:flex; flex-direction:column; align-items:center; justify-content:center; height:100vh; background:#0f1118; color:#fff; font-family:'Space Grotesk', sans-serif;">
							<span class="material-symbols-outlined" style="font-size:4.5rem; color:#ef4444; margin-bottom:16px;">power_off</span>
							<h1 style="margin:0 0 8px 0; font-size:1.9rem;">V-CoPanel Shut Down</h1>
							<p style="color:rgba(255,255,255,0.5); margin:0;">All background daemons, shared services, and databases have been terminated safely.<br>You may now close this window.</p>
						</div>
					`;
				}, 800);
			} catch (e) {
				console.error("Shutdown error:", e);
			}
		}
	);
}

let currentConfirmCallback = null;

function showConfirmModal(title, description, onConfirm) {
	const modal = document.getElementById('global-confirm-modal');
	const titleEl = document.getElementById('global-confirm-title');
	const descEl = document.getElementById('global-confirm-desc');
	const btnEl = document.getElementById('global-confirm-btn');
	if (!modal) return;

	if (titleEl) titleEl.innerText = title || 'Confirm Operation';
	if (descEl) descEl.innerHTML = description || 'Are you sure you want to perform this operation?';
	
	currentConfirmCallback = onConfirm;
	
	if (btnEl) {
		btnEl.onclick = () => {
			const cb = currentConfirmCallback;
			closeConfirmModal();
			if (typeof cb === 'function') {
				cb();
			}
		};
	}

	modal.style.display = 'flex';
}

function closeConfirmModal() {
	const modal = document.getElementById('global-confirm-modal');
	if (modal) modal.style.display = 'none';
	currentConfirmCallback = null;
}

window.SystemServices = [];

async function updateDynamicLinks() {
	try {
		const res = await fetch('/api/system/services');
		if (!res.ok) return;
		const data = await res.json();
		if (data.status === 'ok' && data.services) {
			window.SystemServices = data.services;
			data.services.forEach(srv => {
				if (srv.service_key === 'phpmyadmin') {
					const pmaLink = document.getElementById('pma-webui-link');
					if (pmaLink) {
						pmaLink.href = srv.url;
						pmaLink.innerHTML = `<span class="material-symbols-outlined">open_in_new</span> Open phpMyAdmin UI (${srv.port})`;
					}
					const pmaInnerTxt = document.getElementById('txt-pma-inner');
					if (pmaInnerTxt && srv.status === 'running') {
						pmaInnerTxt.innerText = srv.port;
					}
				}
				if (srv.service_key === 'mailpit') {
					const mailpitLink = document.getElementById('mailpit-webui-link');
					if (mailpitLink) {
						mailpitLink.href = srv.url;
						mailpitLink.innerHTML = `<span class="material-symbols-outlined">open_in_new</span> Open Web UI (${srv.port})`;
					}
					const mailpitTxt = document.getElementById('txt-mailpit');
					if (mailpitTxt && srv.status === 'running') {
						mailpitTxt.innerText = srv.port;
					}
				}
			});
		}
	} catch (e) {
		console.error("Failed to update dynamic links:", e);
	}
}

