function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

function el(id) {
    return document.getElementById(id);
}

function setStatus(text, opts = {}) {
    const statusEl = el("bootStatus");
    if (!statusEl) return;
    statusEl.innerText = text;
    statusEl.classList.toggle("error", Boolean(opts.error));
}

function setActionsVisible(visible) {
    const actions = el("bootActions");
    if (!actions) return;
    actions.classList.toggle("visible", Boolean(visible));
}

async function waitForFrameLoad(frame, timeoutMs) {
    return new Promise((resolve) => {
        let done = false;
        const finish = () => {
            if (done) return;
            done = true;
            resolve();
        };

        const t = setTimeout(finish, timeoutMs);
        frame.addEventListener(
            "load",
            () => {
                clearTimeout(t);
                finish();
            },
            { once: true },
        );
    });
}

let bootRunning = false;
async function boot() {
    if (bootRunning) return;

    const frame = el("frame");
    if (!frame) return;

    bootRunning = true;
    setActionsVisible(false);

    try {
        let url = "http://127.0.0.1:7777";

        setStatus("Resolving daemon URL…");
        try {
            url = await window.go.main.App.DaemonURL();
        } catch (err) {
            setStatus(`Failed to get daemon URL: ${String(err)}`, { error: true });
            setActionsVisible(true);
            return;
        }

        setStatus(`Waiting for daemon… (${url})`);
        let ok = false;
        try {
            ok = await window.go.main.App.WaitForDaemon(30000);
        } catch (err) {
            setStatus(`Health check failed: ${String(err)}`, { error: true });
            setActionsVisible(true);
            return;
        }

        if (!ok) {
            setStatus(`Daemon not reachable: ${url}`, { error: true });
            setActionsVisible(true);
            return;
        }

        setStatus(`Loading UI… (${url})`);
        frame.src = url;
        await waitForFrameLoad(frame, 8000);
        await sleep(150);
        el("boot")?.classList.add("hidden");
    } finally {
        bootRunning = false;
    }
}

el("retryButton")?.addEventListener("click", () => {
    boot().catch((err) => console.error(err));
});

boot().catch((err) => {
    console.error(err);
});
