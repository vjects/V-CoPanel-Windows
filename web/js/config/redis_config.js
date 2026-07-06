/* ─── V-CoPanel Redis / Valkey Cache Configuration Engine (redis_config.js) ─── */

async function loadRedisConfig() {
	const badge = document.getElementById('redis-status-badge');
	const memEl = document.getElementById('redis-mem-use');
	const cliEl = document.getElementById('redis-clients-count');
	const keysEl = document.getElementById('redis-keys-count');
	const startBtn = document.getElementById('btn-redis-start');
	const stopBtn = document.getElementById('btn-redis-stop');
	const flushBtn = document.getElementById('btn-redis-flush');

	try {
		const res = await fetch('/api/redis/status');
		const data = await res.json();

		if (!data || !data.installed) {
			if (badge) {
				badge.innerText = 'Not Installed';
				badge.style.background = 'rgba(239,68,68,0.15)';
				badge.style.color = '#f87171';
			}
			if (memEl) memEl.innerText = 'N/A';
			if (cliEl) cliEl.innerText = '0';
			if (keysEl) keysEl.innerText = '0';
			if (startBtn) startBtn.disabled = true;
			if (stopBtn) stopBtn.disabled = true;
			if (flushBtn) flushBtn.disabled = true;
			return;
		}

		if (data.running) {
			if (badge) {
				badge.innerText = 'Running (PONG)';
				badge.style.background = 'rgba(52,180,120,0.15)';
				badge.style.color = 'rgb(52,180,120)';
			}
			if (memEl) memEl.innerText = data.memory_use || '0 B';
			if (cliEl) cliEl.innerText = data.connected_clients || '1';
			if (keysEl) keysEl.innerText = data.total_keys || '0';
			if (startBtn) {
				startBtn.style.display = 'none';
				startBtn.disabled = false;
				startBtn.innerHTML = '<span class="material-symbols-outlined">play_arrow</span> Start Server';
			}
			if (stopBtn) {
				stopBtn.style.display = 'inline-flex';
				stopBtn.disabled = false;
				stopBtn.innerHTML = '<span class="material-symbols-outlined">stop_circle</span> Stop Server';
			}
			if (flushBtn) {
				flushBtn.disabled = false;
				flushBtn.style.opacity = '1';
			}
		} else {
			if (badge) {
				badge.innerText = 'Stopped';
				badge.style.background = 'rgba(250,172,80,0.15)';
				badge.style.color = 'rgb(250,172,80)';
			}
			if (memEl) memEl.innerText = '0 B';
			if (cliEl) cliEl.innerText = '0';
			if (keysEl) keysEl.innerText = '0';
			if (startBtn) {
				startBtn.style.display = 'inline-flex';
				startBtn.disabled = false;
				startBtn.innerHTML = '<span class="material-symbols-outlined">play_arrow</span> Start Server';
			}
			if (stopBtn) {
				stopBtn.style.display = 'none';
				stopBtn.disabled = false;
				stopBtn.innerHTML = '<span class="material-symbols-outlined">stop_circle</span> Stop Server';
			}
			if (flushBtn) {
				flushBtn.disabled = true;
				flushBtn.style.opacity = '0.5';
			}
		}

		try {
			const cfgRes = await fetch('/api/redis/config/get');
			if (cfgRes.ok) {
				const cfgData = await cfgRes.json();
				const memSel = document.getElementById('prm-redis-maxmem');
				const polSel = document.getElementById('prm-redis-policy');
				if (memSel && cfgData.max_memory) memSel.value = cfgData.max_memory;
				if (polSel && cfgData.policy) polSel.value = cfgData.policy;
			}
		} catch (eCfg) {}
	} catch (e) {
		console.error('Error loading Redis status:', e);
		if (badge) {
			badge.innerText = 'Error';
			badge.style.background = 'rgba(239,68,68,0.15)';
			badge.style.color = '#f87171';
		}
	}
}

async function toggleRedis(action) {
	const startBtn = document.getElementById('btn-redis-start');
	const stopBtn = document.getElementById('btn-redis-stop');

	if (action === 'start' && startBtn) {
		startBtn.disabled = true;
		startBtn.innerHTML = '<span class="material-symbols-outlined">hourglass_empty</span> Starting...';
	}
	if (action === 'stop' && stopBtn) {
		stopBtn.disabled = true;
		stopBtn.innerHTML = '<span class="material-symbols-outlined">hourglass_empty</span> Stopping...';
	}

	try {
		await fetch(`/api/redis/${action}`);
		setTimeout(loadRedisConfig, 1000);
	} catch (e) {
		console.error(`Failed to ${action} Redis:`, e);
		loadRedisConfig();
	}
}

async function flushRedisCache() {
	const flushBtn = document.getElementById('btn-redis-flush');
	if (flushBtn) {
		flushBtn.disabled = true;
		const origHtml = flushBtn.innerHTML;
		flushBtn.innerHTML = '<span class="material-symbols-outlined">hourglass_empty</span> Flushing...';
		try {
			await fetch('/api/redis/flush', { method: 'POST' });
			setTimeout(() => {
				flushBtn.innerHTML = '<span class="material-symbols-outlined">check</span> Cache Flushed!';
				setTimeout(() => {
					flushBtn.innerHTML = origHtml;
					flushBtn.disabled = false;
					loadRedisConfig();
				}, 1500);
			}, 500);
		} catch (e) {
			console.error('Failed to flush Redis:', e);
			flushBtn.innerHTML = origHtml;
			flushBtn.disabled = false;
		}
	}
}

function copyRedisConfig(lang, btnElement) {
	let content = '';
	if (lang === 'laravel') {
		content = `REDIS_CLIENT=phpredis
REDIS_HOST=127.0.0.1
REDIS_PASSWORD=null
REDIS_PORT=6379
REDIS_DB=0
CACHE_DRIVER=redis
QUEUE_CONNECTION=redis
SESSION_DRIVER=redis`;
	} else if (lang === 'node') {
		content = `{
  "host": "127.0.0.1",
  "port": 6379,
  "password": null,
  "db": 0,
  "keyPrefix": "app:",
  "lazyConnect": false
}`;
	} else if (lang === 'go') {
		content = `// go-redis/v9 Options
rdb := redis.NewClient(&redis.Options{
	Addr:	  "127.0.0.1:6379",
	Password: "", // no password set
	DB:		  0,  // use default DB
})`;
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

function switchRedisConfigTab(lang) {
	const tabs = ['laravel', 'node', 'go'];
	tabs.forEach(t => {
		const block = document.getElementById(`redis-block-${t}`);
		const btn = document.getElementById(`redis-tab-btn-${t}`);
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

async function saveRedisTuning() {
	const memSel = document.getElementById('prm-redis-maxmem');
	const polSel = document.getElementById('prm-redis-policy');
	const mem = memSel ? memSel.value : '256mb';
	const pol = polSel ? polSel.value : 'allkeys-lru';

	try {
		const res = await fetch('/api/redis/config/update', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ max_memory: mem, policy: pol })
		});
		if (!res.ok) {
			if (typeof notifier !== 'undefined' && notifier.error) notifier.error('Error', 'Failed to save Redis tuning parameters.');
			else if (typeof showToast === 'function') showToast('Error', 'Failed to save Redis tuning parameters.', 'error');
		}
	} catch (e) {
		console.error('Error saving Redis tuning:', e);
		if (typeof notifier !== 'undefined' && notifier.error) notifier.error('Error', 'Network error while saving Redis config.');
	}
}
