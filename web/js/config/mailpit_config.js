/* ─── V-CoPanel Mailpit SMTP & Mailbox Testing Engine (mailpit_config.js) ─── */

async function loadMailpitConfig() {
	if (typeof updateDynamicLinks === 'function') {
		await updateDynamicLinks();
	}
	const badge = document.getElementById('mailpit-status-badge');
	const startBtn = document.getElementById('btn-mailpit-start');
	const stopBtn = document.getElementById('btn-mailpit-stop');
	const clearBtn = document.getElementById('btn-mailpit-clear');
	const webuiLink = document.getElementById('mailpit-webui-link');

	try {
		const res = await fetch('/api/mailpit/status');
		const data = await res.json();

		if (data && data.running) {
			if (badge) {
				const srv = window.SystemServices ? window.SystemServices.find(s => s.service_key === 'mailpit') : null;
				const port = srv ? srv.port : '8025';
				badge.innerText = `Running (Port ${port})`;
				badge.style.background = 'rgba(52,180,120,0.15)';
				badge.style.color = 'rgb(52,180,120)';
			}
			if (startBtn) {
				startBtn.style.display = 'none';
				startBtn.disabled = false;
				startBtn.innerHTML = '<span class="material-symbols-outlined">play_arrow</span> Launch Mailpit Server';
			}
			if (stopBtn) {
				stopBtn.style.display = 'inline-flex';
				stopBtn.disabled = false;
				stopBtn.innerHTML = '<span class="material-symbols-outlined">stop_circle</span> Stop Server';
			}
			if (clearBtn) {
				clearBtn.disabled = false;
				clearBtn.style.opacity = '1';
			}
			if (webuiLink) {
				webuiLink.style.display = 'inline-flex';
				webuiLink.style.opacity = '1';
				webuiLink.style.pointerEvents = 'auto';
			}
		} else {
			if (badge) {
				badge.innerText = 'Stopped (Idle)';
				badge.style.background = 'rgba(250,172,80,0.15)';
				badge.style.color = 'rgb(250,172,80)';
			}
			if (startBtn) {
				startBtn.style.display = 'inline-flex';
				startBtn.disabled = false;
				startBtn.innerHTML = '<span class="material-symbols-outlined">play_arrow</span> Launch Mailpit Server';
			}
			if (stopBtn) {
				stopBtn.style.display = 'none';
				stopBtn.disabled = false;
				stopBtn.innerHTML = '<span class="material-symbols-outlined">stop_circle</span> Stop Server';
			}
			if (clearBtn) {
				clearBtn.disabled = true;
				clearBtn.style.opacity = '0.5';
			}
			if (webuiLink) {
				webuiLink.style.opacity = '0.5';
				webuiLink.style.pointerEvents = 'none';
			}
		}
	} catch (e) {
		console.error('Error loading Mailpit status:', e);
		if (badge) {
			badge.innerText = 'Error';
			badge.style.background = 'rgba(239,68,68,0.15)';
			badge.style.color = '#f87171';
		}
	}
}

// Keep backward compatible alias
async function checkMailpitStatus() {
	await loadMailpitConfig();
}

async function startMailpit() {
	const btn = document.getElementById('btn-mailpit-start');
	if (btn) {
		btn.disabled = true;
		btn.innerHTML = '<span class="material-symbols-outlined">hourglass_empty</span> Launching...';
	}
	try {
		await fetch('/api/mailpit/start', { method: 'POST' });
		setTimeout(loadMailpitConfig, 1000);
	} catch (e) {
		console.error('Failed to start Mailpit:', e);
		loadMailpitConfig();
	}
}

async function stopMailpit() {
	const btn = document.getElementById('btn-mailpit-stop');
	if (btn) {
		btn.disabled = true;
		btn.innerHTML = '<span class="material-symbols-outlined">hourglass_empty</span> Stopping...';
	}
	try {
		await fetch('/api/mailpit/stop', { method: 'POST' });
		setTimeout(loadMailpitConfig, 1000);
	} catch (e) {
		console.error('Failed to stop Mailpit:', e);
		loadMailpitConfig();
	}
}

async function clearMailbox() {
	const clearBtn = document.getElementById('btn-mailpit-clear');
	if (clearBtn) {
		clearBtn.disabled = true;
		const origHtml = clearBtn.innerHTML;
		clearBtn.innerHTML = '<span class="material-symbols-outlined">hourglass_empty</span> Clearing Inbox...';
		try {
			const res = await fetch('/api/mailpit/clear', { method: 'POST' });
			if (res.ok) {
				clearBtn.innerHTML = '<span class="material-symbols-outlined">check</span> Inbox Cleared!';
			} else {
				clearBtn.innerHTML = '<span class="material-symbols-outlined">error</span> Error';
			}
			setTimeout(() => {
				clearBtn.innerHTML = origHtml;
				clearBtn.disabled = false;
				loadMailpitConfig();
			}, 1500);
		} catch (e) {
			console.error('Failed to clear mailbox:', e);
			clearBtn.innerHTML = origHtml;
			clearBtn.disabled = false;
		}
	}
}

function copyMailpitConfig(lang, btnElement) {
	let content = '';
	if (lang === 'laravel') {
		content = `MAIL_MAILER=smtp
MAIL_HOST=127.0.0.1
MAIL_PORT=1025
MAIL_USERNAME=null
MAIL_PASSWORD=null
MAIL_ENCRYPTION=null
MAIL_FROM_ADDRESS="hello@v-copanel.test"
MAIL_FROM_NAME="\${APP_NAME}"`;
	} else if (lang === 'node') {
		content = `// Node.js (Nodemailer) Transport Config
const nodemailer = require('nodemailer');

const transporter = nodemailer.createTransport({
  host: '127.0.0.1',
  port: 1025,
  secure: false, // TLS/SSL disabled for local testing
  ignoreTLS: true
});`;
	} else if (lang === 'go') {
		content = `// Go (net/smtp) Plain Auth & Send
addr := "127.0.0.1:1025"
from := "hello@v-copanel.test"
to := []string{"dev@v-copanel.test"}
msg := []byte("Subject: Test Email\\r\\n\\r\\nThis is a Mailpit test.")

err := smtp.SendMail(addr, nil, from, to, msg)`;
	}

	if (typeof copyText === 'function') {
		copyText(content, btnElement);
	} else {
		navigator.clipboard.writeText(content);
		if (btnElement) {
			const originalHtml = btnElement.innerHTML;
			btnElement.innerHTML = '<span class="material-symbols-outlined">check</span> Copied!';
			setTimeout(() => { btnElement.innerHTML = originalHtml; }, 2000);
		}
	}
}

function switchMailpitConfigTab(lang) {
	const tabs = ['laravel', 'node', 'go'];
	tabs.forEach(t => {
		const block = document.getElementById(`mailpit-block-${t}`);
		const btn = document.getElementById(`mailpit-tab-btn-${t}`);
		if (block) block.style.display = (t === lang) ? 'block' : 'none';
		if (btn) {
			if (t === lang) {
				btn.classList.add('active');
				btn.style.background = 'rgba(255,255,255,0.15)';
				btn.style.borderColor = 'rgba(255,255,255,0.3)';
				btn.style.color = '#ffffff';
			} else {
				btn.classList.remove('active');
				btn.style.background = 'transparent';
				btn.style.borderColor = 'rgba(255,255,255,0.08)';
				btn.style.color = 'rgba(255,255,255,0.6)';
			}
		}
	});
}
