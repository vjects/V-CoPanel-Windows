/* ─── Bridge Core Studio Toast System ───────────────────────────────────────
   Studio Glass aesthetics with smooth slide-in animations & accent borders.
──────────────────────────────────────────────────────────────────────────── */

let lastToastFetchTime = 0;

function ensureToastContainer() {
	let container = document.getElementById('bridge-toast-container');
	if (!container) {
		container = document.createElement('div');
		container.id = 'bridge-toast-container';
		container.style.cssText = `
			position: fixed;
			bottom: 24px;
			right: 24px;
			z-index: 99999;
			display: flex;
			flex-direction: column;
			gap: 12px;
			max-width: 380px;
			width: calc(100vw - 48px);
			pointer-events: none;
		`;
		document.body.appendChild(container);

		// Inject animations
		const style = document.createElement('style');
		style.textContent = `
			@keyframes toastSlideIn {
				from { opacity: 0; transform: translateX(40px) scale(0.96); }
				to { opacity: 1; transform: translateX(0) scale(1); }
			}
			@keyframes toastSlideOut {
				from { opacity: 1; transform: translateX(0) scale(1); }
				to { opacity: 0; transform: translateX(40px) scale(0.96); }
			}
			.bridge-toast-item {
				pointer-events: auto;
				background: linear-gradient(145deg, rgba(28,32,44,0.88) 0%, rgba(15,17,23,0.92) 100%);
				backdrop-filter: blur(24px);
				border: 1px solid rgba(255,255,255,0.1);
				border-radius: 14px;
				padding: 14px 16px;
				box-shadow: 0 10px 30px rgba(0,0,0,0.5);
				display: flex;
				align-items: flex-start;
				gap: 12px;
				animation: toastSlideIn 0.28s cubic-bezier(0.16, 1, 0.3, 1) forwards;
				position: relative;
				overflow: hidden;
				transition: all 0.2s ease;
			}
			.bridge-toast-item:hover {
				border-color: rgba(255,255,255,0.2);
			}
		`;
		document.head.appendChild(style);
	}
	return container;
}

function showToast(title, message, type = 'info', duration = 5000) {
	const container = ensureToastContainer();

	const colors = {
		success: { accent: '#10b981', icon: 'check_circle', bg: 'rgba(16,185,129,0.12)' },
		error:   { accent: '#ef4444', icon: 'error',        bg: 'rgba(239,68,68,0.12)' },
		warning: { accent: '#f59e0b', icon: 'warning',      bg: 'rgba(245,158,11,0.12)' },
		info:    { accent: '#3b82f6', icon: 'info',         bg: 'rgba(59,130,246,0.12)' }
	};
	const cfg = colors[type] || colors.info;

	const toastEl = document.createElement('div');
	toastEl.className = 'bridge-toast-item';
	toastEl.style.borderLeft = `4px solid ${cfg.accent}`;

	toastEl.innerHTML = `
		<div style="width:36px; height:36px; border-radius:10px; background:${cfg.bg}; display:flex; align-items:center; justify-content:center; flex-shrink:0;">
			<span class="material-symbols-outlined" style="color:${cfg.accent}; font-size:1.3rem;">${cfg.icon}</span>
		</div>
		<div style="flex:1; min-width:0;">
			<div style="font-weight:700; color:#ffffff; font-size:0.92rem; line-height:1.3;">${title}</div>
			<div style="color:rgba(255,255,255,0.6); font-size:0.82rem; margin-top:3px; line-height:1.4; word-break:break-word;">${message}</div>
		</div>
		<button onclick="this.closest('.bridge-toast-item').remove()" style="background:none; border:none; color:rgba(255,255,255,0.35); cursor:pointer; padding:2px; display:flex; align-items:center;">
			<span class="material-symbols-outlined" style="font-size:1.1rem;">close</span>
		</button>
	`;

	container.appendChild(toastEl);

	if (duration > 0) {
		setTimeout(() => {
			if (toastEl.parentNode) {
				toastEl.style.animation = 'toastSlideOut 0.25s forwards';
				setTimeout(() => toastEl.remove(), 250);
			}
		}, duration);
	}
}

// Background polling for server-side generated notifications
async function pollServerToasts() {
	try {
		if (lastToastFetchTime === 0) {
			lastToastFetchTime = Date.now();
			return;
		}
		const res = await fetch(`/api/system/toasts?since=${lastToastFetchTime}`);
		if (!res.ok) return;
		const list = await res.json();
		if (Array.isArray(list) && list.length > 0) {
			list.forEach(t => {
				showToast(t.title, t.message, t.type, t.duration || 5000);
				if (t.created_at > lastToastFetchTime) {
					lastToastFetchTime = t.created_at;
				}
			});
		}
	} catch (e) {}
}

setInterval(pollServerToasts, 2000);

// Intercept browser alert() to render as Studio Glass Toast
window._origAlert = window.alert;
window.alert = function(msg) {
	if (typeof showToast === 'function') {
		showToast('System Notice', String(msg), 'info', 6000);
	} else if (window._origAlert) {
		window._origAlert(msg);
	}
};

// Global Clipboard Copy utility with micro animations and feedback toast
function copyText(text, btn) {
	navigator.clipboard.writeText(text).then(() => {
		if (typeof showToast === 'function') {
			showToast('Copied!', `"${text}" copied to clipboard.`, 'success', 2500);
		}
		if (btn) {
			const origHtml = btn.innerHTML;
			btn.innerHTML = `<span class="material-symbols-outlined" style="font-size:1rem; color:#10b981; margin-right:4px;">check</span> Copied`;
			btn.style.borderColor = '#10b981';
			btn.style.color = '#10b981';
			setTimeout(() => {
				btn.innerHTML = origHtml;
				btn.style.borderColor = '';
				btn.style.color = '';
			}, 1500);
		}
	}).catch(err => {
		if (typeof showToast === 'function') {
			showToast('Copy Failed', String(err), 'error', 3000);
		}
	});
}
