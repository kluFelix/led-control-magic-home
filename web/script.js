const colorCanvas = document.getElementById('color-canvas');
const ctx = colorCanvas.getContext('2d');

const brightnessSlider = document.getElementById('brightness-value');
const brightnessDisplay = document.getElementById('brightness-display');
const ipAddressInput = document.getElementById('ip-address');

const INSTALL_BUTTON_ID = 'pwa-install-btn';

let isUpdatingFromInputs = false;
let debounceTimer = null;
let isDragging = false;
let currentHue = 0;
let currentSat = 0;
let currentVal = 0;

async function fetchState() {
    try {
        const response = await fetch('/api/state');
        if (response.ok) {
            const state = await response.json();
            currentHue = state.hue !== undefined ? state.hue : 0;
            currentSat = state.saturation !== undefined ? state.saturation : 0;
            currentVal = Math.round((state.value !== undefined ? state.value : 0) * 100);
            brightnessSlider.value = currentVal;
            brightnessDisplay.textContent = currentVal;
            updateColorFromHSL();
            drawColorPicker();
        }
    } catch (e) {
        console.error('Failed to fetch state:', e);
    }
}

async function updateState() {
    try {
        await fetch('/api/state', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                hue: currentHue,
                saturation: currentSat,
                value: currentVal / 100.0,
                ipAddress: ipAddressInput.value.trim()
            })
        });
    } catch (e) {
        console.error('Failed to update state:', e);
    }
}

function resizeCanvas() {
    const rect = colorCanvas.getBoundingClientRect();
    colorCanvas.width = rect.width;
    colorCanvas.height = rect.height;
    drawColorPicker();
}

function updateColorFromHSL() {
    brightnessDisplay.textContent = currentVal;
    const rgb = hslToRgb(currentHue, currentSat / 100, currentVal / 100);
    const glowRgb = hslToRgb(currentHue, currentSat / 100, 1);
    const glowColor = `rgba(${Math.round(glowRgb.r * 255)}, ${Math.round(glowRgb.g * 255)}, ${Math.round(glowRgb.b * 255)}, 0.25)`;
    const sliderRgb = hslToRgb(currentHue, currentSat / 100, 1);
    const sliderColor = `rgb(${Math.round(sliderRgb.r * 255)}, ${Math.round(sliderRgb.g * 255)}, ${Math.round(sliderRgb.b * 255)})`;
    document.documentElement.style.setProperty('--glow-color', glowColor);
    document.documentElement.style.setProperty('--slider-color', sliderColor);
    document.documentElement.style.setProperty('--slider-dark-color', `hsl(${currentHue}, 70%, 35%)`);
    document.documentElement.style.setProperty('--slider-shadow', `${sliderColor}80`);
    document.documentElement.style.setProperty('--slider-shadow-hover', `${sliderColor}cc`);
}

function drawColorPicker() {
    const width = colorCanvas.width;
    const height = colorCanvas.height;

    const pickerMinDisplay = 0.4; // make the picker 50% brighter than the led
    const displayVal = pickerMinDisplay + (currentVal * (1 - pickerMinDisplay)) / 100;

    for (let x = 0; x < width; x++) {
        for (let y = 0; y < height; y++) {
            const h = (x / width) * 360;
            const normalizedY = y / height;
            const sat = 1 - Math.pow(0.01, 1 - normalizedY);
            const v = displayVal;
            const rgb = hslToRgb(h, sat, v);
            ctx.fillStyle = `rgb(${Math.round(rgb.r * 255)}, ${Math.round(rgb.g * 255)}, ${Math.round(rgb.b * 255)})`;
            ctx.fillRect(x, y, 1, 1);
        }
    }

    const gradient = ctx.createLinearGradient(0, 0, 0, height);
    gradient.addColorStop(0, 'rgba(255, 255, 255, 0)');
    gradient.addColorStop(0.5, 'rgba(255, 255, 255, 0.1)');
    gradient.addColorStop(1, 'rgba(255, 255, 255, 1)');
    ctx.globalAlpha = 1;
    ctx.fillStyle = gradient;
    ctx.fillRect(0, 0, width, height);

    const markerX = (currentHue / 360) * width;
    const normalizedY = 1 - Math.log(1 - currentSat / 100) / Math.log(0.01);
    const markerY = normalizedY * height;

    ctx.beginPath();
    ctx.arc(markerX, markerY, 8, 0, Math.PI * 2);
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.9)';
    ctx.lineWidth = 2;
    ctx.stroke();

    ctx.beginPath();
    ctx.arc(markerX, markerY, 5, 0, Math.PI * 2);
    const markerColor = hslToRgb(currentHue, currentSat / 100, currentVal / 100);
    ctx.fillStyle = `rgb(${Math.round(markerColor.r * 255)}, ${Math.round(markerColor.g * 255)}, ${Math.round(markerColor.b * 255)})`;
    ctx.fill();
}

function hslToRgb(h, s, v) {
    const c = v * s;
    const x = c * (1 - Math.abs((h / 60) % 2 - 1));
    const m = v - c;
    let r, g, b;

    if (h < 60) { r = c; g = x; b = 0; }
    else if (h < 120) { r = x; g = c; b = 0; }
    else if (h < 180) { r = 0; g = c; b = x; }
    else if (h < 240) { r = 0; g = x; b = c; }
    else if (h < 300) { r = x; g = 0; b = c; }
    else { r = c; g = 0; b = x; }

    return {
        r: r + m,
        g: g + m,
        b: b + m
    };
}

function sendColorCommand() {
    if (debounceTimer) {
        clearTimeout(debounceTimer);
    }

    debounceTimer = setTimeout(() => {
        const rgb = hslToRgb(currentHue, currentSat / 100, currentVal / 100);
        sendCommand('color', [
            Math.round(rgb.r * 255).toString(16).toUpperCase().padStart(2, '0'),
            Math.round(rgb.g * 255).toString(16).toUpperCase().padStart(2, '0'),
            Math.round(rgb.b * 255).toString(16).toUpperCase().padStart(2, '0')
        ]);
    }, 300);
}

async function sendCommand(command, args = []) {
    const ip = ipAddressInput.value.trim();

    if (!ip) {
        return;
    }

    try {
        let body;

        if (command === 'on') {
            body = { address: ip, on: true };
        } else if (command === 'off') {
            body = { address: ip, on: false };
        } else if (command === 'color') {
            const brightnessInput = document.getElementById('brightness-value');
            const brightness = brightnessInput ? parseInt(brightnessInput.value) : 100;

            if (brightness === 0) {
                body = { address: ip, on: false };
            } else {
                body = {
                    address: ip,
                    color: {
                        r: parseInt(args[0], 16) || 0,
                        g: parseInt(args[1], 16) || 0,
                        b: parseInt(args[2], 16) || 0
                    },
                    brightness: brightness
                };
            }
        }

        const response = await fetch('/api/led', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(body)
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const result = await response.json();
        console.log('Command sent successfully!');

    } catch (error) {
        console.error('Error:', error);
    }
}

function handleCanvasClick(e) {
    const rect = colorCanvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;

    currentHue = (x / colorCanvas.width) * 360;
    const normalizedY = y / colorCanvas.height;
    currentSat = Math.round((1 - Math.pow(0.01, 1 - normalizedY)) * 100);

    updateColorFromHSL();
    drawColorPicker();
    sendColorCommand();
}

function handleCanvasMove(e) {
    if (!isDragging) return;

    const rect = colorCanvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;

    currentHue = (x / colorCanvas.width) * 360;
    const normalizedY = y / colorCanvas.height;
    currentSat = Math.round((1 - Math.pow(0.01, 1 - normalizedY)) * 100);

    updateColorFromHSL();
    drawColorPicker();
    sendColorCommand();
}

function handleCanvasTouch(e) {
    const rect = colorCanvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;

    currentHue = (x / colorCanvas.width) * 360;
    const normalizedY = y / colorCanvas.height;
    currentSat = Math.round((1 - Math.pow(0.01, 1 - normalizedY)) * 100);

    updateColorFromHSL();
    drawColorPicker();
    sendColorCommand();
}

colorCanvas.addEventListener('mousedown', (e) => {
    isDragging = true;
    handleCanvasClick(e);
    updateState();
});

colorCanvas.addEventListener('mousemove', (e) => {
    if (isDragging) {
        handleCanvasMove(e);
        updateState();
    }
});

window.addEventListener('mouseup', () => {
    isDragging = false;
});

colorCanvas.addEventListener('touchstart', (e) => {
    e.preventDefault();
    isDragging = true;
    handleCanvasTouch(e.touches[0]);
    updateState();
}, { passive: false });

colorCanvas.addEventListener('touchmove', (e) => {
    e.preventDefault();
    if (!isDragging) return;
    handleCanvasTouch(e.touches[0]);
    updateState();
}, { passive: false });

colorCanvas.addEventListener('touchend', () => {
    isDragging = false;
});

colorCanvas.addEventListener('touchcancel', () => {
    isDragging = false;
});

brightnessSlider.addEventListener('input', (e) => {
    const value = e.target.value;
    brightnessDisplay.textContent = value;
    currentVal = parseInt(value);
    drawColorPicker();
    updateState();
    sendColorCommand();
});

ipAddressInput.addEventListener('input', () => {
    updateState();
});

window.addEventListener('resize', () => {
resizeCanvas();
});

resizeCanvas();
fetchState();

if ('serviceWorker' in navigator) {
    let deferredPrompt;
    
    window.addEventListener('beforeinstallprompt', (e) => {
        e.preventDefault();
        deferredPrompt = e;
        showInstallButton();
    });

    window.addEventListener('appinstalled', () => {
        deferredPrompt = null;
        hideInstallButton();
    });
}

function showInstallButton() {
    if (document.getElementById(INSTALL_BUTTON_ID)) return;

    const container = document.querySelector('.container');
    const installBtn = document.createElement('button');
    installBtn.id = INSTALL_BUTTON_ID;
    installBtn.className = 'control-btn btn-color';
    installBtn.textContent = '➕ Add to Home Screen';
    installBtn.style.marginTop = '20px';
    installBtn.style.width = '100%';
    
    installBtn.addEventListener('click', async () => {
        if (deferredPrompt) {
            deferredPrompt.prompt();
            const { outcome } = await deferredPrompt.userChoice;
            console.log('Install prompt outcome:', outcome);
            deferredPrompt = null;
            hideInstallButton();
        }
    });

    container.appendChild(installBtn);
}

function hideInstallButton() {
    const btn = document.getElementById(INSTALL_BUTTON_ID);
    if (btn) btn.remove();
}

resizeCanvas();
