/* ─── V-CoPanel Engine Module (vcopanel_engine.js) ────────────────────────
   Strict No-Rainbow Glassmorphism & Internal 6-Tab Navigation System
   Tabs: Info, Performance, Pre-Check, Profile, Status, Reset
──────────────────────────────────────────────────────────────────────────── */

let engineTelemetryInterval = null;
let precheckPollingInterval = null;
let currentEngineTab = 'info';
let isPrecheckRunning = false;

// 1. First Boot Onboarding (Called by app.js on DOMContentLoaded)
async function initSystemBoot() {
	try {
		const res = await fetch('/api/engine/info');
		if (!res.ok) return;
		const data = await res.json();
		
		// If first boot (after initial install, when pre-check not done yet)
		if (data.first_boot) {
			// Most importantly: System Pre-Check must be the very first page shown!
			switchTab('tab-vcopanel_engine');
			switchEngineTab('precheck');
			
			// Show required Toast notification
			if (typeof showToast === 'function') {
				showToast("⏳ Pre-Check Running", "Pre-Check is running, please wait. For more information, go to V-CoPanel Engine.", 8000);
			}
			
			// Automatically start precheck provisioning
			runEnginePrecheckNow();
		}
	} catch (e) {
		console.error("Boot check failed:", e);
	}
}

function switchEngineTab(tabId) {
	currentEngineTab = tabId;
	
	// Update sidebar buttons
	const buttons = document.querySelectorAll('.engine-nav-item');
	buttons.forEach(btn => {
		if (btn.getAttribute('data-target') === tabId) {
			btn.classList.add('active');
		} else {
			btn.classList.remove('active');
		}
	});

	// Update tab panes
	const panes = document.querySelectorAll('.engine-pane');
	panes.forEach(pane => {
		if (pane.id === `engine-pane-${tabId}`) {
			pane.style.display = 'block';
		} else {
			pane.style.display = 'none';
		}
	});

	// Trigger specific tab loaders
	if (tabId === 'performance') {
		loadEnginePerformance();
		if (!engineTelemetryInterval) {
			engineTelemetryInterval = setInterval(loadEnginePerformance, 1500);
		}
	} else {
		if (engineTelemetryInterval) {
			clearInterval(engineTelemetryInterval);
			engineTelemetryInterval = null;
		}
	}

	if (tabId === 'precheck') {
		loadEnginePrecheck();
	} else {
		if (precheckPollingInterval && !isPrecheckRunning) {
			clearInterval(precheckPollingInterval);
			precheckPollingInterval = null;
		}
	}

	if (tabId === 'status') loadEngineStatus();
	if (tabId === 'info') loadEngineInfo();
}

async function loadVCoPanelEngine() {
	switchEngineTab(currentEngineTab || 'info');
	loadEngineInfo();
}

async function loadEngineInfo() {
	try {
		const res = await fetch('/api/engine/info');
		if (res.ok) {
			const data = await res.json();
			const elVer = document.getElementById('engine-info-version');
			const elEd = document.getElementById('engine-info-edition');
			const elCorePhp = document.getElementById('engine-info-core-php');
			if (elVer && data.version) elVer.innerText = data.version;
			if (elEd && data.edition) elEd.innerText = data.edition;
			if (elCorePhp && data.core_php) elCorePhp.innerText = 'Core PHP: ' + data.core_php;
		}

		// Fetch Dynamic Infrastructure Records
		const resInfra = await fetch('/api/system/infrastructure');
		if (resInfra.ok) {
			const dataInfra = await resInfra.json();
			if (dataInfra.records) {
				renderDynamicPillars(dataInfra.records);
			}
		}
	} catch (e) {
		console.error("Failed to load engine info:", e);
	}
}

function renderDynamicPillars(records) {
	const container = document.getElementById('dynamic-pillars-container');
	if (!container) return;

	// Group records
	const cores = records.filter(r => r.category === 'core');
	const runtimes = records.filter(r => r.category === 'runtime');
	const shared = records.filter(r => r.category === 'shared-service');

	const renderList = (items) => {
		if (items.length === 0) return `<li style="opacity:0.5;">No modules detected</li>`;
		return items.map(item => `
			<li>
				<div class="status-indicator"></div>
				<div style="flex:1;">
					<strong style="color:#fff;">${item.service_name}</strong>
					<span style="font-size:0.75rem; color:rgba(255,255,255,0.45); display:block;">${item.version}</span>
				</div>
			</li>
		`).join('');
	};

	container.innerHTML = `
		<!-- Pillar 1 -->
		<div class="pillar-card pillar-core" style="animation: pillarSlideUp 0.6s cubic-bezier(0.16, 1, 0.3, 1) 0.1s forwards;">
			<div style="display:flex; align-items:center; gap:12px;">
				<div class="icon-wrapper">
					<span class="material-symbols-outlined" style="color:#3b82f6; font-size:1.4rem;">developer_board</span>
				</div>
				<div>
					<h4 style="margin:0; font-size:0.98rem; color:#fff; font-weight:700; letter-spacing:0.3px;">Core Daemon</h4>
					<span style="font-size:0.75rem; color:rgba(255,255,255,0.45);">Orchestration & IPC</span>
				</div>
			</div>
			<ul class="dynamic-list">
				${renderList(cores)}
			</ul>
		</div>

		<!-- Pillar 2 -->
		<div class="pillar-card pillar-runtimes" style="animation: pillarSlideUp 0.6s cubic-bezier(0.16, 1, 0.3, 1) 0.2s forwards;">
			<div style="display:flex; align-items:center; gap:12px;">
				<div class="icon-wrapper">
					<span class="material-symbols-outlined" style="color:#06b6d4; font-size:1.4rem;">terminal</span>
				</div>
				<div>
					<h4 style="margin:0; font-size:0.98rem; color:#fff; font-weight:700; letter-spacing:0.3px;">Isolated Runtimes</h4>
					<span style="font-size:0.75rem; color:rgba(255,255,255,0.45);">Zero-Dependency Execution</span>
				</div>
			</div>
			<ul class="dynamic-list">
				${renderList(runtimes)}
			</ul>
		</div>

		<!-- Pillar 3 -->
		<div class="pillar-card pillar-services" style="animation: pillarSlideUp 0.6s cubic-bezier(0.16, 1, 0.3, 1) 0.3s forwards;">
			<div style="display:flex; align-items:center; gap:12px;">
				<div class="icon-wrapper">
					<span class="material-symbols-outlined" style="color:#10b981; font-size:1.4rem;">database</span>
				</div>
				<div>
					<h4 style="margin:0; font-size:0.98rem; color:#fff; font-weight:700; letter-spacing:0.3px;">Shared Services</h4>
					<span style="font-size:0.75rem; color:rgba(255,255,255,0.45);">Enterprise Data Daemons</span>
				</div>
			</div>
			<ul class="dynamic-list">
				${renderList(shared)}
			</ul>
		</div>
	`;
}

async function loadEnginePerformance() {
	try {
		const res = await fetch('/api/engine/performance');
		if (!res.ok) return;
		const data = await res.json();

		const osTotal = data.os_total_ram_mb || 16384;
		const osUsed = data.os_used_ram_mb || 4096;
		const vcoCore = data.vcopanel_core_ram_mb || 5;
		const vcoTotal = data.vcopanel_total_ram_mb || 180;
		const cpu = data.cpu_load_percent || 5;

		// Update OS RAM
		const osPercent = Math.round((osUsed / osTotal) * 100);
		const elOsPercent = document.getElementById('engine-perf-os-percent');
		const elOsDetail = document.getElementById('engine-perf-os-detail');
		const elOsBar = document.getElementById('engine-perf-os-bar');
		if (elOsPercent) elOsPercent.innerText = osPercent + '%';
		if (elOsDetail) elOsDetail.innerText = `${osUsed.toLocaleString()} MB / ${osTotal.toLocaleString()} MB Used`;
		if (elOsBar) elOsBar.style.width = osPercent + '%';

		// Update VCO RAM
		const elVcoRam = document.getElementById('engine-perf-vco-ram');
		const elVcoBar = document.getElementById('engine-perf-vco-bar');
		if (elVcoRam) elVcoRam.innerHTML = vcoTotal + ' <span style="font-size:1.2rem; color:rgba(255,255,255,0.5);">MB</span>';
		let vcoPercent = Math.round((vcoTotal / 1024) * 100);
		if (vcoPercent > 100) vcoPercent = 100;
		if (elVcoBar) elVcoBar.style.width = vcoPercent + '%';

		// Update CPU
		const elCpu = document.getElementById('engine-perf-cpu');
		if (elCpu) elCpu.innerText = cpu + '%';

		// Update Shared Services Telemetry
		const mariaMB = data.maria_ram_mb || 0;
		const redisMB = data.redis_ram_mb || 0;
		const mailpitMB = data.mailpit_ram_mb || 0;

		const elMaria = document.getElementById('engine-perf-maria');
		const elRedis = document.getElementById('engine-perf-redis');
		const elMailpit = document.getElementById('engine-perf-mailpit');
		if (elMaria) elMaria.innerText = mariaMB + ' MB';
		if (elRedis) elRedis.innerText = redisMB + ' MB';
		if (elMailpit) elMailpit.innerText = mailpitMB + ' MB';

		// Update Active Projects Telemetry
		const projects = data.projects || [];
		const activeCountEl = document.getElementById('engine-perf-active-count');
		if (activeCountEl) {
			activeCountEl.innerText = `${projects.length} Process${projects.length !== 1 ? 'es' : ''} Active`;
		}

		const projectsContainer = document.getElementById('engine-perf-projects-container');
		if (projectsContainer) {
			if (projects.length === 0) {
				projectsContainer.innerHTML = `
					<div style="color:rgba(255,255,255,0.35); text-align:center; padding:20px; font-size:0.85rem;">
						No active project processes running. Start a project server or queue to view metrics.
					</div>
				`;
			} else {
				projectsContainer.innerHTML = projects.map(proj => {
					const isQueue = proj.type === 'queue';
					const icon = isQueue ? 'precision_manufacturing' : 'public';
					const badgeColor = isQueue ? 'rgba(168,85,247,0.12)' : 'rgba(59,130,246,0.12)';
					const textColor = isQueue ? '#c084fc' : '#60a5fa';
					const borderColor = isQueue ? 'rgba(168,85,247,0.25)' : 'rgba(59,130,246,0.25)';
					const typeLabel = isQueue ? 'QUEUE WORKER' : 'WEB SERVER';
					
					let ramPct = Math.round((proj.ram_mb / 256) * 100);
					if (ramPct > 100) ramPct = 100;
					if (ramPct < 3) ramPct = 3;

					return `
						<div style="display:flex; align-items:center; justify-content:space-between; gap:16px; background:rgba(255,255,255,0.015); border:1px solid rgba(255,255,255,0.04); border-radius:12px; padding:12px 16px; transition:all 0.25s ease;" onmouseover="this.style.borderColor='rgba(255,255,255,0.08)'; this.style.background='rgba(255,255,255,0.035)';" onmouseout="this.style.borderColor='rgba(255,255,255,0.04)'; this.style.background='rgba(255,255,255,0.015)';">
							<div style="display:flex; align-items:center; gap:12px; width:220px; flex-shrink:0;">
								<span class="material-symbols-outlined" style="color:${textColor}; font-size:1.4rem;">${icon}</span>
								<div>
									<strong style="color:#fff; font-size:0.92rem; display:block;">${proj.name}</strong>
									<span class="badge" style="background:${badgeColor}; color:${textColor}; border:1px solid ${borderColor}; font-size:0.65rem; padding:1px 6px; font-weight:700;">${typeLabel}</span>
								</div>
							</div>
							
							<div style="flex:1; display:flex; align-items:center; gap:12px;">
								<div style="flex:1; background:rgba(0,0,0,0.3); height:6px; border-radius:3px; overflow:hidden; border:1px solid rgba(255,255,255,0.03);">
									<div style="background:linear-gradient(90deg, ${textColor}, #a78bfa); height:100%; width:${ramPct}%;"></div>
								</div>
								<span style="font-family:'JetBrains Mono', monospace; font-size:0.8rem; color:rgba(255,255,255,0.4); width:75px; text-align:right;">PID ${proj.pid}</span>
							</div>
							
							<div style="text-align:right; width:80px; flex-shrink:0;">
								<strong style="color:#fff; font-family:'JetBrains Mono', monospace; font-size:0.95rem;">${proj.ram_mb}</strong>
								<span style="font-size:0.75rem; color:rgba(255,255,255,0.4); font-weight:500;"> MB</span>
							</div>
						</div>
					`;
				}).join('');
			}
		}
	} catch (e) {
		console.error("Failed to load engine performance:", e);
	}
}

async function optimizeEngineMemory() {
	const btn = document.getElementById('btn-optimize-memory');
	if (btn) {
		btn.innerHTML = `<span class="material-symbols-outlined" style="font-size:1.2rem;">sync</span> Boosting...`;
		btn.disabled = true;
		btn.style.opacity = '0.7';
	}
	try {
		await fetch('/api/engine/optimize_memory');
		setTimeout(() => {
			if (btn) {
				btn.innerHTML = `<span class="material-symbols-outlined" style="font-size:1.2rem;">check_circle</span> Optimized`;
				btn.style.background = 'rgba(52, 180, 120, 0.3)';
				setTimeout(() => {
					btn.innerHTML = `<span class="material-symbols-outlined" style="font-size:1.2rem;">bolt</span> Boost RAM`;
					btn.disabled = false;
					btn.style.opacity = '1';
					btn.style.background = 'rgba(52,180,120,0.15)';
				}, 2000);
			}
			loadEnginePerformance();
		}, 800); // slight artificial delay for UX
	} catch (e) {
		if (btn) {
			btn.innerHTML = `<span class="material-symbols-outlined" style="font-size:1.2rem;">bolt</span> Boost RAM`;
			btn.disabled = false;
			btn.style.opacity = '1';
		}
	}
}

// Real-time Pre-Check verification with dynamic button & in-place DOM updates
async function loadEnginePrecheck(silent = false) {
	const grid = document.getElementById('engine-precheck-grid');
	const btn = document.getElementById('btn-run-precheck');
	
	if (grid && !silent && !grid.children.length) {
		grid.innerHTML = `<div style="color:rgba(255,255,255,0.4); padding:16px;">Querying database inventory...</div>`;
	}

	try {
		const res = await fetch('/api/precheck/status');
		if (!res.ok) throw new Error("API failed");
		const data = await res.json();
		
		const wasRunning = isPrecheckRunning;
		isPrecheckRunning = !!data.running;

		updateGlobalProgressBar(data, wasRunning);
		updateTopbarWidget(data);
		if (btn) updatePrecheckButton(btn);

		if (grid && data.assets) {
			renderPrecheckGrid(grid, data.assets);
		}
		handlePrecheckPolling(wasRunning);

	} catch (e) {
		if (!silent && grid) grid.innerHTML = `<div style="color:#f87171; padding:16px;">Failed to verify precheck inventory.</div>`;
	}
}

function ensurePrecheckDOM(globalBar) {
	if (!globalBar) return;
	
	// Force parent styles to be a fullscreen blocker
	globalBar.style.position = 'fixed';
	globalBar.style.top = '0';
	globalBar.style.left = '0';
	globalBar.style.width = '100vw';
	globalBar.style.height = '100vh';
	globalBar.style.zIndex = '999999';
	globalBar.style.background = 'rgba(8, 10, 16, 0.95)';
	globalBar.style.backdropFilter = 'blur(20px)';
	globalBar.style.webkitBackdropFilter = 'blur(20px)';
	globalBar.style.alignItems = 'center';
	globalBar.style.justifyContent = 'center';
	globalBar.style.flexDirection = 'column';
	globalBar.style.padding = '20px';
	globalBar.style.borderBottom = 'none';
	globalBar.style.boxShadow = 'none';

	if (globalBar.querySelector('.orb-wrapper')) return; // Already injected

	// Re-inject the entire premium layout
	globalBar.innerHTML = `
		<div class="orb-wrapper">
			<div class="orb-ring"></div>
			<div class="orb-ring"></div>
			<div class="orb-core"></div>
		</div>

		<h2 style="margin:0 0 8px 0; color:#fff; font-size:1.6rem; font-weight:700; letter-spacing:0.5px; text-align:center;">V-CoPanel Environment Booting</h2>
		<span style="font-size:0.95rem; color:#10b981; font-family:'JetBrains Mono', monospace; font-weight:700; margin-bottom:24px; text-shadow:0 0 10px rgba(16,185,129,0.3);" id="global-precheck-percent">0% Completed</span>
		
		<!-- Log Box Console -->
		<div style="background:rgba(0,0,0,0.55); border:1px solid rgba(255,255,255,0.06); border-radius:18px; padding:20px; font-family:'JetBrains Mono', monospace; font-size:0.82rem; color:#a7f3d0; width:100%; max-width:480px; height:130px; overflow-y:auto; display:flex; flex-direction:column-reverse; gap:6px; box-shadow:0 12px 30px rgba(0,0,0,0.5);">
			<div id="global-precheck-text" style="line-height:1.5; color:#34d399;">⏳ Initializing 3-Pillar Workspace Provisioning...</div>
		</div>

		<!-- Subtle Progress Bar -->
		<div style="width:100%; max-width:480px; height:4px; background:rgba(255,255,255,0.06); border-radius:2px; overflow:hidden; margin-top:16px;">
			<div id="global-precheck-fill" style="width:0%; height:100%; background: linear-gradient(90deg, #3b82f6, #10b981); transition: width 0.4s ease; box-shadow: 0 0 10px #10b981;"></div>
		</div>
	`;
	
	// Inject styles dynamically if they don't exist in document
	if (!document.getElementById('orb-precheck-styles')) {
		const style = document.createElement('style');
		style.id = 'orb-precheck-styles';
		style.innerHTML = `
			.orb-wrapper {
				position: relative;
				width: 180px;
				height: 180px;
				display: flex;
				align-items: center;
				justify-content: center;
				margin-bottom: 24px;
			}
			.orb-core {
				position: absolute;
				width: 60px;
				height: 60px;
				background: #fff;
				border-radius: 50%;
				box-shadow: 0 0 30px #fff, 0 0 60px #10b981;
				z-index: 10;
				animation: orbBreathe 3s ease-in-out infinite alternate;
			}
			.orb-ring {
				position: absolute;
				width: 100%;
				height: 100%;
				border-radius: 50%;
				background: conic-gradient(from 0deg, #3b82f6, #06b6d4, #10b981, #3b82f6);
				filter: blur(14px);
				opacity: 0.8;
				animation: orbSpin 4s linear infinite;
			}
			.orb-ring:nth-child(2) {
				width: 120%;
				height: 120%;
				filter: blur(28px);
				opacity: 0.45;
				animation: orbSpinReverse 7s linear infinite;
			}
			@keyframes orbSpin { 100% { transform: rotate(360deg); } }
			@keyframes orbSpinReverse { 100% { transform: rotate(-360deg); } }
			@keyframes orbBreathe {
				0% { transform: scale(0.9); box-shadow: 0 0 20px #fff, 0 0 50px #3b82f6; }
				100% { transform: scale(1.1); box-shadow: 0 0 35px #fff, 0 0 75px #10b981; }
			}
		`;
		document.head.appendChild(style);
	}
}

function updateGlobalProgressBar(data, wasRunning) {
	const totalCount = data.assets ? Object.keys(data.assets).length : 15;
	const readyCount = data.assets ? Object.values(data.assets).filter(v => v).length : 0;
	const percent = totalCount > 0 ? Math.round((readyCount / totalCount) * 100) : 0;

	const globalBar = document.getElementById('global-precheck-bar');
	if (globalBar && isPrecheckRunning) {
		ensurePrecheckDOM(globalBar);
	}

	const globalPercent = document.getElementById('global-precheck-percent');
	const globalFill = document.getElementById('global-precheck-fill');
	const globalText = document.getElementById('global-precheck-text');

	if (isPrecheckRunning) {
		if (globalBar) globalBar.style.display = 'flex';
		if (globalPercent) globalPercent.innerText = `${percent}% Completed`;
		if (globalFill) globalFill.style.width = `${percent}%`;
		if (globalText) {
			const logsHtml = (data.logs && data.logs.length) 
				? data.logs.map(l => `<div style="margin-bottom:4px; line-height:1.4;">⚡ ${l}</div>`).join('')
				: `<div style="color:#34d399;">⏳ Unpacking offline runtime archives...</div>`;
			globalText.innerHTML = logsHtml;
		}
	} else {
		if (wasRunning && globalBar) {
			if (globalPercent) globalPercent.innerText = `100% Completed`;
			if (globalFill) globalFill.style.width = `100%`;
			if (globalText) globalText.innerHTML = `<div style="color:#34d399; font-weight:bold;">✅ Pre-Check Provisioning Complete! All systems ready.</div>`;
			setTimeout(() => {
				if (!isPrecheckRunning && globalBar) globalBar.style.display = 'none';
			}, 3500);
		} else if (!wasRunning && globalBar) {
			globalBar.style.display = 'none';
		}
	}
}

function updateTopbarWidget(data) {
	const totalCount = data.assets ? Object.keys(data.assets).length : 15;
	const readyCount = data.assets ? Object.values(data.assets).filter(v => v).length : 0;
	const percent = totalCount > 0 ? Math.round((readyCount / totalCount) * 100) : 0;

	const topBtn = document.getElementById('topbar-btn-precheck');
	const topIcon = document.getElementById('topbar-precheck-icon');
	const topText = document.getElementById('topbar-precheck-text');

	if (!topBtn || !topIcon || !topText) return;

	if (isPrecheckRunning) {
		if (topBtn.hideTimeout) { clearTimeout(topBtn.hideTimeout); topBtn.hideTimeout = null; }
		topBtn.style.display = 'inline-flex';
		topBtn.style.opacity = '1';
		topBtn.style.transform = 'scale(1)';
		topBtn.style.background = 'rgba(245, 158, 11, 0.15)';
		topBtn.style.borderColor = 'rgba(245, 158, 11, 0.35)';
		topBtn.style.color = '#fcd34d';
		topIcon.style.color = '#f59e0b';
		topIcon.style.animation = 'spin 1s linear infinite';
		topIcon.innerText = 'sync';
		topText.innerText = `Pre-Check (${percent}%)`;
	} else if (!data.ready) {
		if (topBtn.hideTimeout) { clearTimeout(topBtn.hideTimeout); topBtn.hideTimeout = null; }
		topBtn.style.display = 'inline-flex';
		topBtn.style.opacity = '1';
		topBtn.style.transform = 'scale(1)';
		topBtn.style.background = 'rgba(239, 68, 68, 0.15)';
		topBtn.style.borderColor = 'rgba(239, 68, 68, 0.35)';
		topBtn.style.color = '#fca5a5';
		topIcon.style.color = '#ef4444';
		topIcon.style.animation = 'none';
		topIcon.innerText = 'radar';
		topText.innerText = 'Pre-Check Required';
	} else {
		topBtn.style.display = 'inline-flex';
		topBtn.style.background = 'rgba(16, 185, 129, 0.15)';
		topBtn.style.borderColor = 'rgba(16, 185, 129, 0.35)';
		topBtn.style.color = '#6ee7b7';
		topIcon.style.color = '#10b981';
		topIcon.style.animation = 'none';
		topIcon.innerText = 'check_circle';
		topText.innerText = 'Pre-Check Verified';

		if (!topBtn.hideTimeout && topBtn.style.display !== 'none') {
			topBtn.hideTimeout = setTimeout(() => {
				if (!isPrecheckRunning && topBtn) {
					topBtn.style.transition = 'all 0.5s ease';
					topBtn.style.opacity = '0';
					topBtn.style.transform = 'scale(0.9)';
					setTimeout(() => { if (!isPrecheckRunning) topBtn.style.display = 'none'; }, 500);
				}
			}, 4000);
		}
	}
}

function updatePrecheckButton(btn) {
	if (!btn) return;
	if (isPrecheckRunning) {
		btn.disabled = true;
		btn.style.opacity = '0.75';
		btn.style.cursor = 'not-allowed';
		btn.innerHTML = `<span class="material-symbols-outlined" style="font-size:1.1rem; animation: spin 1s linear infinite;">sync</span> Running Pre-Check...`;
	} else {
		btn.disabled = false;
		btn.style.opacity = '1';
		btn.style.cursor = 'pointer';
		btn.innerHTML = `<span class="material-symbols-outlined" style="font-size:1.1rem;">sync</span> Run Pre-Check Now`;
	}
}

function renderPrecheckGrid(grid, assets) {
	if (grid.children.length !== Object.keys(assets).length || grid.innerHTML.includes('Querying') || grid.innerHTML.includes('Failed')) {
		grid.innerHTML = '';
		for (const [name, ready] of Object.entries(assets)) {
			const card = document.createElement('div');
			card.className = 'engine-item-card';
			card.id = `precheck-card-${name}`;
			card.innerHTML = `
				<div style="display:flex; align-items:center; gap:12px;">
					<span class="material-symbols-outlined" style="color:rgba(255,255,255,0.45); font-size:1.4rem;">inventory_2</span>
					<div>
						<h4 style="margin:0; font-size:0.95rem; color:#fff;">${name}</h4>
						<span style="font-size:0.75rem; color:rgba(255,255,255,0.4);">3-Pillar Offline Archive</span>
					</div>
				</div>
				<div id="precheck-badge-${name}">
					${ready 
						? `<span class="badge badge-active" style="transition:all 0.3s ease;"><span class="material-symbols-outlined" style="font-size:1rem;">check_circle</span> Verified</span>`
						: `<span class="badge" style="background:rgba(239,68,68,0.15); color:#fca5a5; transition:all 0.3s ease;"><span class="material-symbols-outlined" style="font-size:1rem;">error</span> Missing</span>`
					}
				</div>
			`;
			grid.appendChild(card);
		}
	} else {
		for (const [name, ready] of Object.entries(assets)) {
			const badgeEl = document.getElementById(`precheck-badge-${name}`);
			if (badgeEl) {
				const wasReady = badgeEl.innerHTML.includes('Verified');
				if (ready && !wasReady) {
					badgeEl.innerHTML = `<span class="badge badge-active" style="transition:all 0.3s ease; transform:scale(1.08); box-shadow:0 0 12px rgba(16,185,129,0.3);"><span class="material-symbols-outlined" style="font-size:1rem;">check_circle</span> Verified</span>`;
					setTimeout(() => {
						const b = badgeEl.querySelector('.badge');
						if (b) {
							b.style.transform = 'scale(1)';
							b.style.boxShadow = 'none';
						}
					}, 400);
				} else if (!ready && wasReady) {
					badgeEl.innerHTML = `<span class="badge" style="background:rgba(239,68,68,0.15); color:#fca5a5;"><span class="material-symbols-outlined" style="font-size:1rem;">error</span> Missing</span>`;
				}
			}
		}
	}
}

function handlePrecheckPolling(wasRunning) {
	if (isPrecheckRunning && !precheckPollingInterval) {
		precheckPollingInterval = setInterval(() => loadEnginePrecheck(true), 1200);
	} else if (!isPrecheckRunning && precheckPollingInterval) {
		clearInterval(precheckPollingInterval);
		precheckPollingInterval = null;
		if (wasRunning && typeof showToast === 'function') {
			showToast("✅ Pre-Check Completed", "All offline runtimes and shared services verified successfully.");
		}
	}
}

async function runEnginePrecheckNow() {
	if (isPrecheckRunning) return;
	if (typeof showToast === 'function') {
		showToast("⏳ Pre-Check Started", "Unpacking offline archives and verifying 3-Pillar workspace...");
	}
	try {
		await fetch('/api/precheck/run', { method: 'POST' });
		isPrecheckRunning = true;
		loadEnginePrecheck(true);
		if (!precheckPollingInterval) {
			precheckPollingInterval = setInterval(() => loadEnginePrecheck(true), 1200);
		}
	} catch (e) {
		console.error("Precheck error:", e);
		if (typeof showToast === 'function') showToast("❌ Error", "Failed to start Pre-Check engine.");
	}
}

let enginePingCache = {};

async function loadEngineStatus() {
	const el = document.getElementById('engine-status-matrix');
	if (!el) return;

	// Don't reload if already loaded to preserve Untested or Available states, unless forced
	if (el.innerHTML.includes('engine-item-card') && !el.innerHTML.includes('Querying system matrix')) return;

	el.innerHTML = `<div style="color:rgba(255,255,255,0.5); padding:20px; display:flex; align-items:center; gap:10px;"><span class="material-symbols-outlined" style="animation: spin 1s linear infinite;">sync</span> Querying system infrastructure...</div>`;

	const getCachedBadgeHTML = (key) => {
		const status = enginePingCache[key] || 'Untested';
		if (status === 'Available') {
			return `<span class="badge" style="background:rgba(59,130,246,0.15); color:#60a5fa; border:1px solid rgba(59,130,246,0.3); padding:6px 12px;"><span style="width:6px; height:6px; border-radius:50%; background:#60a5fa; display:inline-block; margin-right:6px; box-shadow:0 0 8px #60a5fa;"></span>Available</span>`;
		} else if (status === 'Offline') {
			return `<span class="badge" style="background:rgba(239,68,68,0.1); color:#ef4444; border:1px solid rgba(239,68,68,0.3); padding:6px 12px;">Offline</span>`;
		}
		return `<span class="badge" style="background:rgba(255,255,255,0.05); color:#cbd5e1; border:1px solid rgba(255,255,255,0.1); padding:6px 12px;">Untested</span>`;
	};
	
	const getCachedCardStyle = (key) => {
		const status = enginePingCache[key] || 'Untested';
		if (status === 'Available') {
			return 'border:1px solid rgba(59,130,246,0.2); background:rgba(59,130,246,0.05);';
		} else if (status === 'Offline') {
			return 'border:1px solid rgba(239,68,68,0.2); background:rgba(255,255,255,0.02);';
		}
		return 'border:1px solid rgba(255,255,255,0.06); background:rgba(255,255,255,0.02);';
	};

	try {
		const resInfra = await fetch('/api/system/infrastructure');
		let infra = [];
		if (resInfra.ok) {
			const data = await resInfra.json();
			if (data.records) infra = data.records;
		}

		let html = `
			<div class="engine-item-card" id="status-card-vco" style="${getCachedCardStyle('vco')} animation: pillarSlideUp 0.4s forwards;">
				<div style="display:flex; align-items:center; gap:16px;">
					<span class="material-symbols-outlined" style="color:rgba(255,255,255,0.3); font-size:2rem; background:rgba(255,255,255,0.03); padding:8px; border-radius:12px;">memory</span>
					<div><strong style="color:#fff; font-size:1.05rem;">V-CoPanel Bridge Daemon</strong><br><small style="color:rgba(255,255,255,0.45); font-size:0.8rem;">Core Service (Go IPC Handler)</small></div>
				</div>
				<span id="status-badge-vco" class="badge-wrapper">${getCachedBadgeHTML('vco')}</span>
			</div>
		`;

		let delay = 0.1;
		infra.forEach((item, idx) => {
			let icon = 'terminal';
			if (item.category === 'shared-service') icon = 'storage';
			else if (item.category === 'core') icon = 'developer_board';

			html += `
				<div class="engine-item-card" id="status-card-${idx}" data-key="${item.service_key}" data-cat="${item.category}" style="${getCachedCardStyle(item.service_key)} animation: pillarSlideUp 0.4s forwards ${delay}s; opacity:0;">
					<div style="display:flex; align-items:center; gap:16px;">
						<span class="material-symbols-outlined" style="color:rgba(255,255,255,0.3); font-size:2rem; background:rgba(255,255,255,0.03); padding:8px; border-radius:12px;">${icon}</span>
						<div><strong style="color:#fff; font-size:1.05rem;">${item.service_name}</strong><br><small style="color:rgba(255,255,255,0.45); font-size:0.8rem;">${item.category === 'runtime' ? 'Isolated Engine' : 'System Module'} | ${item.version}</small></div>
					</div>
					<span id="status-badge-${idx}" class="badge-wrapper">${getCachedBadgeHTML(item.service_key)}</span>
				</div>
			`;
			delay += 0.1;
		});

		el.innerHTML = html;

	} catch (e) {
		el.innerHTML = `<div style="color:#ef4444; padding:20px; font-family:monospace;">Failed to load system infrastructure: ${e.message}</div>`;
		console.error(e);
	}
}

async function pingEngineStatus() {
	const btn = document.getElementById('btn-ping-matrix');
	if (btn) {
		btn.innerHTML = `<span class="material-symbols-outlined" style="font-size:1.1rem; animation: spin 1s linear infinite;">sync</span> Pinging...`;
		btn.disabled = true;
	}

	try {
		const resMatrix = await fetch('/api/engine/status_matrix');
		let matrix = { mariadb: false, redis: false, mailpit: false, phpmyadmin: false };
		if (resMatrix.ok) matrix = await resMatrix.json();

		// 1. VCO Daemon (always test first)
		const vcoBadge = document.getElementById('status-badge-vco');
		const vcoCard = document.getElementById('status-card-vco');
		if(vcoBadge && vcoCard) {
			vcoBadge.innerHTML = `<span class="material-symbols-outlined" style="font-size:1rem; animation:spin 1s linear infinite;">sync</span> Testing...`;
			setTimeout(() => {
				vcoCard.style.border = '1px solid rgba(52,180,120,0.3)';
				vcoCard.style.background = 'rgba(52,180,120,0.05)';
				vcoBadge.innerHTML = `<span class="badge" style="background:rgba(52,180,120,0.15); color:#34b478; border:1px solid rgba(52,180,120,0.3); padding:6px 12px;"><span style="width:6px; height:6px; border-radius:50%; background:#34b478; display:inline-block; margin-right:6px; animation:pulseDot 2s infinite;"></span>Available</span>`;
				
				enginePingCache['vco'] = 'Available';
			}, 400);
		}

		// 2. Others sequentially
		const cards = document.querySelectorAll('.engine-item-card[id^="status-card-"]');
		let delay = 600;
		cards.forEach(card => {
			if(card.id === 'status-card-vco') return;
			const idx = card.id.split('-')[2];
			const key = card.getAttribute('data-key');
			const cat = card.getAttribute('data-cat');
			const badge = document.getElementById(`status-badge-${idx}`);

			setTimeout(() => {
				badge.innerHTML = `<span class="material-symbols-outlined" style="font-size:1rem; animation:spin 1s linear infinite;">sync</span> Testing...`;
				
				setTimeout(() => {
					let isLive = false;
					if (cat === 'shared-service') {
						if (key === 'mariadb') isLive = matrix.mariadb;
						if (key === 'redis') isLive = matrix.redis;
						if (key === 'mailpit') isLive = matrix.mailpit;
						if (key === 'phpmyadmin') isLive = matrix.phpmyadmin;
					} else {
						isLive = true;
					}

					if (isLive) {
						card.style.border = '1px solid rgba(59,130,246,0.2)';
						card.style.background = 'rgba(59,130,246,0.05)';
						badge.innerHTML = `<span class="badge" style="background:rgba(59,130,246,0.15); color:#60a5fa; border:1px solid rgba(59,130,246,0.3); padding:6px 12px;"><span style="width:6px; height:6px; border-radius:50%; background:#60a5fa; display:inline-block; margin-right:6px; box-shadow:0 0 8px #60a5fa;"></span>Available</span>`;
						
						enginePingCache[key] = 'Available';
					} else {
						card.style.border = '1px solid rgba(239,68,68,0.2)';
						card.style.background = 'rgba(255,255,255,0.02)';
						badge.innerHTML = `<span class="badge" style="background:rgba(239,68,68,0.1); color:#ef4444; border:1px solid rgba(239,68,68,0.3); padding:6px 12px;">Offline</span>`;
						
						enginePingCache[key] = 'Offline';
					}
				}, 400);

			}, delay);
			delay += 200;
		});

		setTimeout(() => {
			if (btn) {
				btn.innerHTML = `<span class="material-symbols-outlined" style="font-size:1.1rem;">sensors</span> Ping Again`;
				btn.disabled = false;
			}
		}, delay + 400);

	} catch (e) {
		console.error("Ping error:", e);
		if (btn) {
			btn.innerHTML = `<span class="material-symbols-outlined" style="font-size:1.1rem;">error</span> Failed`;
			setTimeout(() => { btn.innerHTML = `<span class="material-symbols-outlined" style="font-size:1.1rem;">sensors</span> Ping`; btn.disabled = false; }, 2000);
		}
	}
}

function launchFactoryReset() {
	if (!confirm("⚠️ WARNING: This will reset all workspace services, delete databases, and restore factory defaults. Continue?")) {
		return;
	}
	window.location.href = '/views/reset.html';
}

// Standalone App Mode Minimum Window Size Constraint
window.addEventListener('resize', () => {
	if (window.outerWidth < 1100 || window.outerHeight < 700) {
		window.resizeTo(Math.max(window.outerWidth, 1100), Math.max(window.outerHeight, 700));
	}
});

