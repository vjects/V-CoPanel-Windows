/* ─── V-CoPanel Topbar Dynamic Context Module (topbar.js) ──────────────────
   Fetches /api/ui/topbar-context (DB-driven) and updates ALL topbar elements.
   No hardcoded strings. No hardcoded paths. No emoji decorations.
   Follows Persona: Check & Run — reads real state before rendering.
──────────────────────────────────────────────────────────────────────────── */

async function loadTopbarContext() {
	try {
		const res = await fetch('/api/ui/topbar-context');
		if (!res.ok) return;
		const ctx = await res.json();

		_updateTopbarIdentity(ctx);
		_updateTopbarServices(ctx.services || []);
		_updateTopbarProjectChip(ctx);
	} catch (e) {
		console.error('[Topbar] Failed to load context:', e);
	}
}

function _updateTopbarIdentity(ctx) {
	const nameEl = document.getElementById('topbar-engine-name');
	const verEl  = document.getElementById('topbar-engine-version');
	if (nameEl && ctx.engine_name) nameEl.innerText = ctx.engine_name;
	if (verEl  && ctx.engine_version) verEl.innerText = ctx.engine_version;
}

function _updateTopbarServices(services) {
	// Map of service_key -> { port, tabId }
	const config = {
		mariadb:    { dotId: 'dot-mariadb',  txtId: 'txt-mariadb' },
		redis:      { dotId: 'dot-redis',     txtId: 'txt-redis'   },
		mailpit:    { dotId: 'dot-mailpit',   txtId: 'txt-mailpit' },
		phpmyadmin: { dotId: 'dot-pma',       txtId: 'txt-pma'     },
	};

	services.forEach(svc => {
		const ids = config[svc.key];
		if (!ids) return;

		const dot = document.getElementById(ids.dotId);
		const txt = document.getElementById(ids.txtId);
		if (!dot || !txt) return;

		if (svc.running) {
			dot.style.background   = '#10b981';
			dot.style.boxShadow    = '0 0 10px #10b981';
			dot.style.animation    = 'pulse-green 2s infinite';
			txt.style.color        = '#10b981';
			txt.innerText          = svc.port;
		} else {
			dot.style.background   = '#ef4444';
			dot.style.boxShadow    = 'none';
			dot.style.animation    = 'pulse-red 3s infinite';
			txt.style.color        = 'rgba(255,255,255,0.4)';
			txt.innerText          = 'Stopped';
		}
	});
}

function _updateTopbarProjectChip(ctx) {
	const chip        = document.getElementById('topbar-project-chip');
	const countEl     = document.getElementById('topbar-project-count');
	const runningWrap = document.getElementById('topbar-running-count');
	const runningVal  = document.getElementById('topbar-running-val');

	if (!chip || !countEl) return;

	const total   = ctx.total_projects   || 0;
	const running = ctx.running_projects || 0;

	if (total > 0) {
		chip.style.display = 'inline-flex';
		countEl.innerText  = total;
		if (running > 0 && runningWrap && runningVal) {
			runningWrap.style.display = 'inline';
			runningVal.innerText      = running;
		} else if (runningWrap) {
			runningWrap.style.display = 'none';
		}
	} else {
		chip.style.display = 'none';
	}
}
