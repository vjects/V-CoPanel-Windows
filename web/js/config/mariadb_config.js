/* ─── V-CoPanel MariaDB & PhpMyAdmin Configuration Engine (mariadb_config.js) ─── */

async function loadMariaDBConfig() {
	checkPMAInnerStatus();
	try {
		const res = await fetch('/api/mariadb/config/get');
		if (res.ok) {
			const data = await res.json();
			const bufSel = document.getElementById('prm-innodb-buffer');
			const connSel = document.getElementById('prm-max-conn');
			const charSel = document.getElementById('prm-charset');
			const collSel = document.getElementById('prm-collation');
			if (bufSel && data.buffer_pool) bufSel.value = data.buffer_pool;
			if (connSel && data.max_connections) connSel.value = data.max_connections;
			if (charSel && data.charset) charSel.value = data.charset;
			if (collSel && data.collation) collSel.value = data.collation;
		}
	} catch (e) {}
}

async function startPMA() {
	const btn = document.getElementById('btn-pma-start');
	if (btn) {
		btn.disabled = true;
		btn.innerHTML = '<span class="material-symbols-outlined" style="animation: spin 1s linear infinite; vertical-align:middle;">sync</span> Starting...';
	}
	
	fetch('/api/phpmyadmin/start', { method: 'POST' }).then(() => {
		setTimeout(checkPMAInnerStatus, 1500); // Give PHP time to bind the port
	}).catch(e => {
		console.error('Failed to start phpMyAdmin:', e);
		checkPMAInnerStatus();
	});
}

async function stopPMA() {
	try {
		await fetch('/api/phpmyadmin/stop', { method: 'POST' });
	} catch (e) {
		console.error('Failed to stop phpMyAdmin:', e);
	}
	checkPMAInnerStatus();
}

async function checkPMAInnerStatus() {
	if (typeof updateDynamicLinks === 'function') {
		await updateDynamicLinks();
	}
	try {
		const res = await fetch('/api/phpmyadmin/status');
		const data = await res.json();
		const dot = document.getElementById('dot-pma-inner');
		const txt = document.getElementById('txt-pma-inner');
		const startBtn = document.getElementById('btn-pma-start');
		const runBtns = document.getElementById('pma-running-buttons');

		if (data && data.running) {
			if (dot) {
				dot.style.background = 'rgb(52,180,120)';
				dot.style.boxShadow = '0 0 8px rgb(52,180,120)';
			}
			if (txt) {
				const srv = window.SystemServices ? window.SystemServices.find(s => s.service_key === 'phpmyadmin') : null;
				txt.innerText = srv ? srv.port : '8881';
				txt.style.color = '#34b478';
			}
			if (startBtn) startBtn.style.display = 'none';
			if (runBtns) runBtns.style.display = 'flex';
		} else {
			if (dot) {
				dot.style.background = 'rgba(255,255,255,0.2)';
				dot.style.boxShadow = 'none';
			}
			if (txt) {
				txt.innerText = 'Offline';
				txt.style.color = 'rgba(255,255,255,0.4)';
			}
			if (startBtn) {
				startBtn.style.display = 'flex';
				startBtn.disabled = false;
				startBtn.innerHTML = '<span class="material-symbols-outlined" style="font-size:1.1rem; margin-right:6px;">play_arrow</span> Start phpMyAdmin Server';
			}
			if (runBtns) runBtns.style.display = 'none';
		}
	} catch (e) {
		console.error("PMA Status error", e);
		const startBtn = document.getElementById('btn-pma-start');
		if (startBtn) {
			startBtn.style.display = 'flex';
			startBtn.disabled = false;
			startBtn.innerHTML = '<span class="material-symbols-outlined" style="font-size:1.1rem; margin-right:6px;">play_arrow</span> Start phpMyAdmin Server';
		}
	}
}

function copyDBConfig(lang, btnElement) {
	let content = '';
	if (lang === 'laravel') {
		content = `DB_CONNECTION=mysql
DB_HOST=127.0.0.1
DB_PORT=3306
DB_DATABASE=your_app_name
DB_USERNAME=admin
DB_PASSWORD=Admin_VCoPanel_2026!`;
	} else if (lang === 'node') {
		content = `{
  "host": "127.0.0.1",
  "port": 3306,
  "user": "admin",
  "password": "Admin_VCoPanel_2026!",
  "database": "your_app_name",
  "connectionLimit": 10,
  "charset": "utf8mb4"
}`;
	} else if (lang === 'go') {
		content = `// database/sql with go-sql-driver/mysql
dsn := "admin:Admin_VCoPanel_2026!@tcp(127.0.0.1:3306)/your_app_name?charset=utf8mb4&parseTime=True&loc=Local"`;
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

function switchDBConfigTab(lang) {
	const tabs = ['laravel', 'node', 'go'];
	tabs.forEach(t => {
		const block = document.getElementById(`db-block-${t}`);
		const btn = document.getElementById(`db-tab-btn-${t}`);
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

async function saveMariaDBTuning() {
	const bufSel = document.getElementById('prm-innodb-buffer');
	const connSel = document.getElementById('prm-max-conn');
	const charSel = document.getElementById('prm-charset');
	const collSel = document.getElementById('prm-collation');
	const buf = bufSel ? bufSel.value : '512M';
	const conn = connSel ? connSel.value : '250';
	const char = charSel ? charSel.value : 'utf8mb4';
	const coll = collSel ? collSel.value : 'utf8mb4_unicode_ci';

	try {
		const res = await fetch('/api/mariadb/config/update', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				buffer_pool: buf,
				max_connections: conn,
				charset: char,
				collation: coll
			})
		});
		if (!res.ok) {
			if (typeof notifier !== 'undefined' && notifier.error) notifier.error('Error', 'Failed to save MariaDB tuning parameters.');
			else if (typeof showToast === 'function') showToast('Error', 'Failed to save MariaDB tuning parameters.', 'error');
		}
	} catch (e) {
		console.error('Error saving MariaDB tuning:', e);
		if (typeof notifier !== 'undefined' && notifier.error) notifier.error('Error', 'Network error while saving MariaDB config.');
	}
}
