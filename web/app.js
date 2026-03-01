const state = {
    user: null,
    usageLocked: false,
    authMode: 'login',
    listings: [],
    current: null,
    selectedId: null,
    versions: [],
    lastText: '',
    uploads: [],
    listingFilter: '',
    styleProfiles: [],
    selectedProfileId: '',
    templates: [],
    selectedTemplateId: '',
    visionAnalysis: null,
    visionDesign: null,
    visionRemix: {
        previewURL: '',
        analysis: null,
        file: null,
        status: '',
        storageURL: '',
        storageKey: '',
    },
    visionRenderImage: '',
    visionRenderAsset: null,
    editingListingId: null,
    annualReport: {
        status: '',
        result: null,
        fileName: '',
        selectedIds: [],
        linkStatus: '',
    },
};

function value(id) {
    return document.getElementById(id)?.value.trim() || '';
}

function numberValue(id) {
    const raw = value(id);
    if (!raw) return 0;
    const parsed = parseFloat(raw);
    return Number.isFinite(parsed) ? parsed : 0;
}

function listFromLines(raw) {
    return raw
        .split(/\r?\n|,/)
        .map(entry => entry.trim())
        .filter(Boolean);
}

function enterAppShell() {
    document.body.classList.add('is-authenticated');
    document.body.classList.remove('is-landing');
    document.body.classList.add('sidebar-open');
}

function enterLandingMode() {
    document.body.classList.add('is-landing');
    document.body.classList.remove('is-authenticated');
    document.body.classList.remove('sidebar-open');
}

function redirectToLanding(message = 'Logga in eller skapa konto.') {
    enterLandingMode();
    hideAuthOverlay();
    showAuthOverlay(message);
    window.scrollTo({ top: 0, behavior: 'instant' });
}

function setUser(user) {
    state.user = user;
    state.usageLocked = isUsageLocked(user);
    const sidebarLabel = document.getElementById('user-email-sidebar');
    if (sidebarLabel) {
        sidebarLabel.textContent = user?.email || 'Inloggad';
    }
    enterAppShell();
    renderSubscriptionStatus();
    if (state.usageLocked) {
        showView('settings');
    }
}

function resetAppData() {
    state.listings = [];
    state.current = null;
    state.selectedId = null;
    state.usageLocked = false;
    renderObjectList();
    renderDetail();
}

function isUsageLocked(user = state.user) {
    if (!user) return false;
    const status = user.subscription_status || '';
    const isPaid = status === 'active' || status === 'trialing';
    const usageCount = user.usage_count || 0;
    return !isPaid && usageCount >= 3;
}

function showAuthOverlay(message = 'Logga in för att fortsätta') {
    const overlay = document.getElementById('auth-overlay');
    const copy = document.getElementById('auth-copy');
    if (copy) copy.textContent = message;
    overlay?.classList.remove('hidden');
}

function hideAuthOverlay() {
    document.getElementById('auth-overlay')?.classList.add('hidden');
    clearAuthError();
}

function closeAuthOverlay() {
    hideAuthOverlay();
    enterLandingMode();
}

function setAuthMode(mode) {
    state.authMode = mode;
    const loginForm = document.getElementById('login-form');
    const registerForm = document.getElementById('register-form');
    const loginTab = document.getElementById('auth-toggle-login');
    const registerTab = document.getElementById('auth-toggle-register');
    loginForm?.classList.toggle('hidden', mode !== 'login');
    registerForm?.classList.toggle('hidden', mode !== 'register');
    loginTab?.classList.toggle('active', mode === 'login');
    registerTab?.classList.toggle('active', mode === 'register');
    clearAuthError();
}


function authCopyForMode(mode) {
    return mode === 'register'
        ? 'Skapa konto eller logga in'
        : 'Logga in';
}

function maybeOpenAuthFromQuery() {
    const params = new URLSearchParams(window.location.search);

    // Handle Stripe checkout return
    const subscriptionResult = params.get('subscription');
    if (subscriptionResult) {
        params.delete('subscription');
        const nextQuery = params.toString();
        const nextURL = `${window.location.pathname}${nextQuery ? `?${nextQuery}` : ''}`;
        window.history.replaceState({}, '', nextURL);
        if (subscriptionResult === 'success' && state.user) {
            // Refresh user to get updated subscription status
            checkSession();
            setTimeout(() => {
                showView('settings');
                alert('Prenumerationen är nu aktiv! Välkommen.');
            }, 500);
            return;
        } else if (subscriptionResult === 'cancelled') {
            // User cancelled checkout
        }
    }

    const raw = (params.get('auth') || '').toLowerCase();
    const mode = raw === 'register' ? 'register' : raw === 'login' ? 'login' : '';
    if (!mode) return;

    params.delete('auth');
    const nextQuery = params.toString();
    const nextURL = `${window.location.pathname}${nextQuery ? `?${nextQuery}` : ''}${window.location.hash || ''}`;
    window.history.replaceState({}, '', nextURL);

    if (state.user) return;
    setAuthMode(mode);
    showAuthOverlay(authCopyForMode(mode));
}

function initLandingMobileMenu() {
    const nav = document.querySelector('.hero__nav');
    const toggle = document.getElementById('hero-menu-toggle');
    const menu = document.getElementById('hero-mobile-menu');
    if (!nav || !toggle || !menu) return;

    const closeMenu = () => {
        nav.classList.remove('is-open');
        toggle.setAttribute('aria-expanded', 'false');
    };

    toggle.addEventListener('click', event => {
        event.stopPropagation();
        const isOpen = nav.classList.toggle('is-open');
        toggle.setAttribute('aria-expanded', isOpen ? 'true' : 'false');
    });

    menu.querySelectorAll('a, button[data-auth-trigger]').forEach(item => {
        item.addEventListener('click', closeMenu);
    });

    document.addEventListener('click', event => {
        if (!nav.contains(event.target)) {
            closeMenu();
        }
    });

    window.addEventListener('resize', () => {
        if (window.innerWidth > 980) {
            closeMenu();
        }
    });
}
function showAuthError(text) {
    const el = document.getElementById('auth-error');
    if (!el) return;
    el.textContent = text || 'Något gick fel';
    el.classList.remove('hidden');
}

function clearAuthError() {
    const el = document.getElementById('auth-error');
    if (!el) return;
    el.textContent = '';
    el.classList.add('hidden');
}

function setupAuthOverlayDismiss() {
    const overlay = document.getElementById('auth-overlay');
    const card = overlay?.querySelector('.auth-card');
    if (!overlay) return;
    overlay.addEventListener('click', (e) => {
        if (e.target === overlay) {
            closeAuthOverlay();
        }
    });
    if (card) {
        card.addEventListener('click', e => e.stopPropagation());
    }
}

async function handleLoginSubmit(e) {
    e.preventDefault();
    clearAuthError();
    const email = document.getElementById('login-email')?.value.trim();
    const password = document.getElementById('login-password')?.value;
    if (!email || !password) {
        showAuthError('Fyll i e-post och lösenord');
        return;
    }
    try {
        const res = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password }),
        });
        if (!res.ok) {
            const msg = await res.text();
            showAuthError(msg || 'Felaktiga uppgifter');
            return;
        }
        const user = await res.json();
        if (user.approved === false) {
            showAuthError('Kontot väntar på godkännande.');
            return;
        }
        setUser(user);
        hideAuthOverlay();
        await initApp();
    } catch (err) {
        showAuthError('Kunde inte logga in just nu');
        console.error(err);
    }
}

async function handleRegisterSubmit(e) {
    e.preventDefault();
    clearAuthError();
    const email = document.getElementById('register-email')?.value.trim();
    const password = document.getElementById('register-password')?.value;
    if (!email || !password) {
        showAuthError('Fyll i e-post och lösenord');
        return;
    }
    try {
        const res = await fetch('/api/auth/register', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password }),
        });
        if (!res.ok) {
            const msg = await res.text();
            showAuthError(msg || 'Kunde inte skapa konto');
            return;
        }
        const user = await res.json();
        if (user.approved === false) {
            showAuthError('Kontot är skapat men väntar på godkännande. Vi hör av oss!');
            return;
        }
        setUser(user);
        hideAuthOverlay();
        await initApp();
    } catch (err) {
        showAuthError('Kunde inte skapa konto just nu');
        console.error(err);
    }
}

async function handleLogout() {
    try {
        await fetch('/api/auth/logout', { method: 'POST' });
    } catch (err) {
        console.warn('logout failed', err);
    }
    state.user = null;
    const sidebarLabel = document.getElementById('user-email-sidebar');
    if (sidebarLabel) sidebarLabel.textContent = 'Ej inloggad';
    resetAppData();
    redirectToLanding('Du är utloggad.');
}

async function checkSession() {
    try {
        const res = await fetch('/api/auth/me');
        if (res.ok) {
            const user = await res.json();
            setUser(user);
            hideAuthOverlay();
            await initApp();
            return;
        }
    } catch (err) {
        console.warn('Session check failed', err);
    }
    enterLandingMode();
    hideAuthOverlay();
}

function handleUnauthorized(message) {
    state.user = null;
    const sidebarLabel = document.getElementById('user-email-sidebar');
    if (sidebarLabel) sidebarLabel.textContent = 'Ej inloggad';
    resetAppData();
    redirectToLanding(message);
}

const nativeFetch = window.fetch.bind(window);
window.fetch = async (input, init = {}) => {
    const res = await nativeFetch(input, { credentials: 'same-origin', ...init });
    if (res.status === 401) {
        handleUnauthorized('Sessionen har gått ut. Logga in igen.');
    }
    if (res.status === 402) {
        handleUsageLimitReached();
    }
    return res;
};

function handleUsageLimitReached() {
    state.usageLocked = true;
    if (state.user) {
        state.user.usage_count = Math.max(state.user.usage_count || 0, 3);
    }
    showView('settings');
    renderSubscriptionStatus();
    renderSidebarUsage();
    const modal = document.createElement('div');
    modal.className = 'usage-limit-modal';
    modal.innerHTML = `
        <div class="usage-limit-card">
            <h2>Gränsen nådd</h2>
            <p>Du har använt dina 3 kostnadsfria AI-anrop.</p>
            <p class="muted">Uppgradera till en prenumeration för obegränsad tillgång till alla AI-funktioner.</p>
            <div class="actions" style="margin-top:1rem;display:flex;gap:0.5rem;">
                <button class="primary" onclick="this.closest('.usage-limit-modal').remove();showView('settings');renderSubscriptionStatus();">Visa prenumerationer</button>
                <button class="ghost" onclick="this.closest('.usage-limit-modal').remove();">Stäng</button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
}

async function fetchListings() {
    try {
        const res = await fetch('/api/listings/');
        if (!res.ok) throw new Error('Kunde inte hämta listor');
        const payload = await res.json();
        state.listings = Array.isArray(payload) ? payload : [];
        if (Array.isArray(state.annualReport.selectedIds)) {
            const valid = new Set(state.listings.map(item => item.id));
            state.annualReport.selectedIds = state.annualReport.selectedIds.filter(id => valid.has(id));
        }
        updateVolumeStats();
        updateTimeSavings();
        updateImageStats();
        renderObjectList();
        renderVisionLab();
        let idToSelect = state.selectedId;
        if (!idToSelect || !state.listings.some(item => item.id === idToSelect)) {
            idToSelect = state.listings[0]?.id || null;
        }
        if (idToSelect) {
            await selectListing(idToSelect);
        } else {
            state.current = null;
            state.versions = [];
            state.lastText = '';
            renderDetail();
        }
    } catch (err) {
        console.error(err);
    }
}

async function fetchStyleProfiles() {
    try {
        const res = await fetch('/api/style-profiles/');
        if (!res.ok) throw new Error('Kunde inte hämta stilprofiler');
        const payload = await res.json();
        state.styleProfiles = Array.isArray(payload) ? payload : [];
        renderStyleProfileOptions();
        renderStyleProfileList();
    } catch (err) {
        console.error(err);
    }
}

async function fetchTemplates() {
    try {
        const res = await fetch('/api/templates/');
        if (!res.ok) throw new Error('Kunde inte hämta mallar');
        const payload = await res.json();
        state.templates = Array.isArray(payload) ? payload : [];
        renderTemplateSelectOptions();
        renderTemplateList();
    } catch (err) {
        console.error(err);
    }
}

function buildPayloadFromForm() {
    const highlights = listFromLines(value('highlights'));
    const images = state.uploads
        .filter(file => file.url && !file.attached)
        .map((file, index) => ({
            url: file.url,
            key: file.key,
            label: file.name,
            source: file.source || 'user',
            kind: file.kind || 'photo',
            cover: index === 0,
        }));
    const payload = {
        address: value('address'),
        neighborhood: value('neighborhood'),
        city: value('city'),
        property_type: value('property-type'),
        rooms: numberValue('rooms'),
        living_area: numberValue('living-area'),
        floor: value('floor'),
        condition: value('condition'),
        association: value('association'),
        balcony: document.getElementById('balcony').checked,
        tone: document.getElementById('tone').value,
        length: document.getElementById('length').value,
        style_profile_id: document.getElementById('style-profile').value || '',
        template_id: document.getElementById('template-select')?.value || '',
        highlights,
        target_audience: 'Bred målgrupp',
        fee: 0,
        images,
    };
    state.selectedProfileId = payload.style_profile_id || '';
    state.selectedTemplateId = payload.template_id || '';
    return payload;
}

async function handleCreate(e) {
    e.preventDefault();
    const payload = buildPayloadFromForm();
    if (!payload.address) {
        alert('Adress krävs.');
        return;
    }
    setFormBusy(true);
    showGenerationLoader();
    try {
        const res = await fetch('/api/listings/', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || 'Misslyckades med att skapa annons');
        }
        const created = await res.json();
        state.selectedId = created.id;
        state.editingListingId = created.id;
        completeGenerationLoader();
        await fetchListings();
        state.uploads = [];
        renderUploads();
        document.getElementById('form-message').textContent = 'Annons genererad.';
        setAIStatus('Klar. Du kan markera texten och omskriva.', false, true);
    } catch (err) {
        alert(err.message);
        setAIStatus('', false, true);
        hideGenerationLoader();
    } finally {
        setFormBusy(false);
    }
}

function setFormBusy(busy) {
    const form = document.getElementById('listing-form');
    form.querySelectorAll('input, select, button').forEach(el => el.disabled = busy);
    const msg = document.getElementById('form-message');
    msg.textContent = busy ? 'Genererar...' : '';
}

async function selectListing(id) {
    if (!id) return;
    state.selectedId = id;
    try {
        const res = await fetch(`/api/listings/${id}/`);
        if (!res.ok) throw new Error('Kunde inte hämta annons');
        state.current = await res.json();
        state.versions = [];
        state.lastText = '';
        renderDetail();
        renderObjectList();
    } catch (err) {
        console.error(err);
    }
}

function renderDetail() {
    const detail = state.current;
    const header = document.getElementById('detail-address');
    const editor = document.getElementById('full-editor');
    const copyBtn = document.getElementById('copy-text-btn');
    const downloadBtn = document.getElementById('download-txt-btn');
    const regenerateBtn = document.getElementById('regenerate-btn');
    const coverEl = document.getElementById('detail-cover');
    const galleryEl = document.getElementById('detail-gallery');

    if (!detail) {
        header.textContent = 'Ingen annons än';
        editor.value = '';
        editor.readOnly = true;
        copyBtn.disabled = true;
        downloadBtn.disabled = true;
        regenerateBtn.disabled = true;
        if (coverEl) coverEl.classList.add('hidden');
        if (galleryEl) galleryEl.classList.add('hidden');
        renderVisionInsights(null);
        renderAnnualInsights(null);
        return;
    }

    const text = getFullCopy(detail);
    header.textContent = detail.address;
    editor.value = text;
    editor.readOnly = false;
    copyBtn.disabled = !text;
    downloadBtn.disabled = !text;
    regenerateBtn.disabled = !text;
    if (coverEl) {
        if (detail.image_url) {
            coverEl.innerHTML = `<img src="${detail.image_url}" alt="Omslagsbild för ${detail.address}">`;
            coverEl.classList.remove('hidden');
        } else {
            coverEl.classList.add('hidden');
            coverEl.innerHTML = '';
        }
    }
    if (galleryEl) {
        const images = detail.details?.media?.images || [];
        if (!images.length) {
            galleryEl.classList.add('hidden');
            galleryEl.innerHTML = '';
        } else {
            galleryEl.classList.remove('hidden');
            galleryEl.innerHTML = images.map(img => `<img src="${img.url}" alt="${img.label || 'Galleri'}">`).join('');
        }
    }

    if (text && text !== state.lastText) {
        pushVersion(text, 'Genererad');
        state.lastText = text;
    }
    renderVersions();
    renderVisionInsights(detail);
    renderAnnualInsights(detail);
}

function getFullCopy(detail) {
    if (detail.full_copy) return detail.full_copy;
    if (detail.sections?.length) {
        return detail.sections.map(sec => sec.content).join('\n\n');
    }
    return '';
}

function renderVisionInsights(detail) {
    const container = document.getElementById('vision-insights');
    const summaryEl = document.getElementById('vision-summary');
    const roomEl = document.getElementById('vision-room');
    const styleEl = document.getElementById('vision-style');
    const tagsEl = document.getElementById('vision-tags');
    if (!container || !summaryEl || !roomEl || !styleEl || !tagsEl) {
        return;
    }

    const vision = detail?.insights?.vision;
    const hasContent = vision && (vision.summary || vision.room_type || vision.style || (vision.notable_details?.length) || (vision.color_palette?.length) || (vision.tags?.length));

    if (!hasContent) {
        container.classList.add('hidden');
        summaryEl.textContent = '';
        roomEl.textContent = '-';
        styleEl.textContent = '';
        styleEl.classList.add('hidden');
        tagsEl.innerHTML = '';
        tagsEl.classList.add('hidden');
        return;
    }

    container.classList.remove('hidden');
    summaryEl.textContent = vision.summary || 'Bildanalysen är klar.';
    roomEl.textContent = vision.room_type || 'Bostadsmiljö';

    if (vision.style) {
        styleEl.textContent = vision.style;
        styleEl.classList.remove('hidden');
    } else {
        styleEl.textContent = '';
        styleEl.classList.add('hidden');
    }

    const badgeValues = [];
    const pushValue = value => {
        const trimmed = (value || '').trim();
        if (trimmed) {
            badgeValues.push(trimmed);
        }
    };

    (vision.notable_details || []).forEach(pushValue);
    (vision.color_palette || []).forEach(color => pushValue(`Färg: ${color}`));
    (vision.tags || []).forEach(pushValue);

    tagsEl.innerHTML = '';
    if (badgeValues.length === 0) {
        tagsEl.classList.add('hidden');
    } else {
        tagsEl.classList.remove('hidden');
        badgeValues.slice(0, 10).forEach(label => {
            const badge = document.createElement('span');
            badge.className = 'vision-badge';
            badge.textContent = label;
            tagsEl.appendChild(badge);
        });
    }
}

function renderAnnualInsights(detail) {
    const container = document.getElementById('annual-insights');
    if (!container) return;
    const report = detail?.details?.association?.annual_report;
    const hasContent = report && Object.values(report).some(val => val && String(val).trim() !== '');
    if (!hasContent) {
        container.classList.add('hidden');
        container.innerHTML = '';
        return;
    }

    const badges = [
        report.source_pages ? `<span class="annual-badge">${report.source_pages} sidor</span>` : '',
        report.characters_analysed ? `<span class="annual-badge">${report.characters_analysed} tecken</span>` : '',
        report.file_name ? `<span class="annual-badge">${report.file_name}</span>` : '',
    ].filter(Boolean).join('');

    const bullets = [
        report.summary ? `<li>${report.summary}</li>` : '',
        report.fee_per_month ? `<li><strong>Avgift:</strong> ${report.fee_per_month}</li>` : '',
        report.debt_per_sqm ? `<li><strong>Skuld/kvm:</strong> ${report.debt_per_sqm}</li>` : '',
        report.total_debt ? `<li><strong>Totala skulder:</strong> ${report.total_debt}</li>` : '',
        report.liquidity ? `<li><strong>Likviditet:</strong> ${report.liquidity}</li>` : '',
        report.planned_maintenance ? `<li><strong>Planerat underhåll:</strong> ${report.planned_maintenance}</li>` : '',
        report.notable_risks ? `<li><strong>Risker:</strong> ${report.notable_risks}</li>` : '',
    ].filter(Boolean).join('');

    const keyLines = [
        ['Org.nr', report.org_number],
        ['Fastighetsbeteckning', report.property_designation],
        ['Byggår', report.build_year],
        ['BOA', report.boa_total],
        ['LOA', report.loa_total],
        ['Skulder till kreditinstitut', report.debt_credit_total],
        ['Kassa & bank', report.cash_and_bank],
        ['Årets resultat', report.net_result],
        ['Räntekostnader', report.interest_costs],
        ['Avskrivningar', report.depreciation],
        ['Intäkter årsavgifter', report.fee_income],
        ['Intäkter lokaler', report.rental_income],
        ['Markägande', report.land_status],
        ['Avgäld utgång', report.land_lease_expiry],
    ].filter(([, val]) => val);

    const reno = [
        report.renovations_done ? `<li><strong>Utfört:</strong> ${report.renovations_done}</li>` : '',
        report.renovations_planned ? `<li><strong>Planerat:</strong> ${report.renovations_planned}</li>` : '',
    ].filter(Boolean).join('');

    container.classList.remove('hidden');
    container.innerHTML = `
        <div class="annual-badges">${badges}</div>
        <div class="annual-grid">
            <div class="annual-card">
                <strong>Årsredovisning</strong>
                <ul>${bullets || '<li>Inga nyckeltal hittades.</li>'}</ul>
            </div>
            <div class="annual-card">
                <strong>Styrelsens kommentarer</strong>
                <p>${report.board_comments || '—'}</p>
            </div>
            <div class="annual-card">
                <strong>Association & ekonomi</strong>
                <ul>
                    ${keyLines.map(([k, v]) => `<li><strong>${k}:</strong> ${v}</li>`).join('') || '<li>—</li>'}
                </ul>
            </div>
            <div class="annual-card">
                <strong>Renoveringar</strong>
                <ul>${reno || '<li>—</li>'}</ul>
            </div>
        </div>
    `;
}

// ── Studio (simplified one-flow image redesign) ──────────────────────────

function openStudioDialog() {
    const overlay = document.getElementById('studio-dialog-overlay');
    const preview = document.getElementById('studio-dialog-preview');
    if (!overlay) return;
    if (state.visionRemix.previewURL) {
        preview.innerHTML = `<img src="${state.visionRemix.previewURL}" alt="Rum">`;
    }
    overlay.classList.remove('hidden');
    setStudioStep(2);
}

function closeStudioDialog() {
    document.getElementById('studio-dialog-overlay')?.classList.add('hidden');
}

/** Update the step indicator bar */
function setStudioStep(activeStep) {
    const steps = document.querySelectorAll('.studio-step');
    steps.forEach(el => {
        const step = parseInt(el.dataset.step, 10);
        el.classList.toggle('done', step < activeStep);
        el.classList.toggle('active', step === activeStep);
    });
}

/** Get the currently selected style from the style card grid */
function getSelectedStyle() {
    const active = document.querySelector('.studio-style-card.active');
    return active?.dataset.style || 'Modern minimalistisk';
}

function setStudioStatus(msg, isError) {
    const el = document.getElementById('studio-dialog-status');
    if (!el) return;
    el.textContent = msg || '';
    el.classList.toggle('error', Boolean(isError));
}

/** Handle file selected via the main dropzone */
function handleStudioFileSelect(event) {
    const file = event.target.files?.[0];
    if (file) processStudioFile(file);
}

function processStudioFile(file) {
    if (!file || !file.type.startsWith('image/')) return;
    state.visionRemix.file = file;
    state.visionRemix.analysis = null;
    state.visionRemix.previewURL = '';
    state.visionRenderImage = '';
    state.visionRenderAsset = null;
    state.visionDesign = null;
    const reader = new FileReader();
    reader.onload = () => {
        state.visionRemix.previewURL = reader.result;
        // Show original immediately, then open the dialog
        renderStudioResult();
        openStudioDialog();
    };
    reader.readAsDataURL(file);
}

/** Handle style card click */
function handleStyleCardClick(event) {
    const card = event.target.closest('.studio-style-card');
    if (!card) return;
    document.querySelectorAll('.studio-style-card').forEach(c => c.classList.remove('active'));
    card.classList.add('active');
}

/** Build aggressive redesign prompt – sent directly to /api/vision/render */
function buildStudioPrompt(style, extra) {
    const parts = [];
    parts.push('⚠️ VIKTIGASTE REGELN – LÄS FÖRST: Ta ALDRIG bort, lägg till, flytta eller dölj något FÖNSTER. Varje fönster i originalfotot MÅSTE finnas kvar på EXAKT samma plats, storlek och form. Denna regel gäller över ALLA andra instruktioner.');
    parts.push('');
    parts.push('🔒 LÅST – FÅR ALDRIG ÄNDRAS:');
    parts.push('1. KAMERAVINKEL – Behåll EXAKT samma kameravinkel, perspektiv, linsförvrängning, komposition och beskärning.');
    parts.push('2. FÖNSTER – Samma antal, storlek, position och form. Har fotot 2 fönster ska resultatet ha EXAKT 2 fönster på samma ställe. ALDRIG ta bort ett fönster.');
    parts.push('');
    parts.push('✅ FÅR OCH SKA ÄNDRAS DRAMATISKT:');
    parts.push('• VÄGGFÄRG – måla om till valfri färg');
    parts.push('• GOLV – byt golvfärg/finish, lägg ny matta');
    parts.push('• ALLA MÖBLER – byt ut varje soffa, säng, bord, stol, hylla mot helt nya');
    parts.push('• GARDINER – byt stil/färg (fönstret bakom ska fortfarande synas)');
    parts.push('• BELYSNING – nya lampor, pendlar, golvlampor');
    parts.push('• TEXTILIER – nya mattor, kuddar, filtar, sängkläder');
    parts.push('• VÄGGDEKOR – ny konst, nya speglar, nya hyllor');
    parts.push('• VÄXTER & TILLBEHÖR – nya växter, böcker, ljus, vaser');
    parts.push('• DÖRRAR – kan byta färg/stil');
    parts.push('• TAK – kan byta färg');
    parts.push('');
    parts.push('STIL: "' + (style || 'Modern minimalistisk') + '"');
    parts.push('');
    parts.push('SÅ HÄR GÖR DU:');
    parts.push('Steg 1: Töm rummet mentalt – ta bort ALLA flyttbara objekt.');
    parts.push('Steg 2: Kontrollera att alla fönster fortfarande finns kvar och är orörda.');
    parts.push('Steg 3: Fyll rummet med en HELT NY inredning. Varje föremål ska vara helt annorlunda.');
    parts.push('Steg 4: RÄKNA fönstren i resultatet och bekräfta att antalet stämmer med originalet.');
    parts.push('');
    parts.push('MINIMUM: huvudmöbel (soffa/säng/matbord), sekundära möbler (soffbord, sidobord, hylla), stor matta, gardiner, 3+ ljuskällor (taklampa + golvlampa + bordslampa), kuddar, 2–3 konstverk, 1–2 växter, dekorativa accessoarer.');
    if (extra) {
        parts.push('');
        parts.push('EXTRA KRAV FRÅN ANVÄNDAREN (obligatoriskt, inte förslag): ' + extra);
    }
    parts.push('');
    parts.push('⚠️ SLUTKONTROLL: Har resultatet SAMMA antal fönster på SAMMA plats som originalet? Om inte, gör om bilden.');
    parts.push('Förändringen ska vara DRAMATISK – som ett professionellt före/efter i en renoveringsserie. Fotorealistiskt, 4K-kvalitet.');
    return parts.join('\n');
}

/** One-click generate: send image + prompt to /api/vision/render */
async function handleStudioGenerate() {
    const promptEl = document.getElementById('vision-design-prompt');
    const genBtn = document.getElementById('studio-generate-btn');
    const labelSpan = genBtn?.querySelector('.studio-generate-btn__label');
    const spinnerSpan = genBtn?.querySelector('.studio-generate-btn__spinner');
    if (!state.visionRemix.previewURL) {
        setStudioStatus('Ingen bild uppladdad.', true);
        return;
    }
    const style = getSelectedStyle();
    const extra = promptEl?.value.trim() || '';
    const prompt = buildStudioPrompt(style, extra);
    setStudioStatus('Genererar ny inredning – kan ta 20–40 sek…', false);
    if (genBtn) genBtn.disabled = true;
    if (labelSpan) labelSpan.textContent = 'Skapar…';
    if (spinnerSpan) spinnerSpan.classList.remove('hidden');

    const baseImageData = (state.visionRemix.previewURL || '').startsWith('data:')
        ? state.visionRemix.previewURL : '';
    try {
        const res = await fetch('/api/vision/render', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                prompt,
                base_image_data: baseImageData,
                base_image_url: '',
            }),
        });
        if (!res.ok) {
            const txt = await res.text();
            throw new Error(txt || 'Kunde inte generera bild');
        }
        const data = await res.json();
        if (data.url) {
            state.visionRenderImage = data.url;
            state.visionRenderAsset = data;
        } else {
            state.visionRenderImage = `data:${data.mime || 'image/png'};base64,${data.data}`;
            state.visionRenderAsset = null;
        }
        setStudioStatus('', false);
        closeStudioDialog();
        setStudioStep(3);
        renderStudioResult();
    } catch (err) {
        state.visionRenderImage = '';
        state.visionRenderAsset = null;
        setStudioStatus(err.message, true);
    } finally {
        if (genBtn) genBtn.disabled = false;
        if (labelSpan) labelSpan.textContent = '\u2728 Skapa ny inredning';
        if (spinnerSpan) spinnerSpan.classList.add('hidden');
    }
}

/** Show / update the comparison view */
function renderStudioResult() {
    const area = document.getElementById('studio-upload-area');
    const result = document.getElementById('studio-result');
    const origImg = document.getElementById('studio-original-img');
    const renderImg = document.getElementById('studio-render-img');
    if (!result) return;

    if (state.visionRemix.previewURL) {
        area?.classList.add('hidden');
        result.classList.remove('hidden');
        if (origImg) origImg.innerHTML = `<img src="${state.visionRemix.previewURL}" alt="Original">`;
    }
    if (state.visionRenderImage) {
        if (renderImg) renderImg.innerHTML = `<img src="${state.visionRenderImage}" alt="AI-designad">`;
    } else {
        if (renderImg) renderImg.innerHTML = '<p class="muted">Bilden visas här efter generering.</p>';
    }

    // Listing select
    const renderSelect = document.getElementById('vision-render-listing');
    if (renderSelect) {
        const prev = renderSelect.value;
        renderSelect.innerHTML = '<option value="">Koppla till objekt…</option>' +
            state.listings.map(item => `<option value="${item.id}">${item.address || 'Namnlöst objekt'}</option>`).join('');
        if (prev && state.listings.some(item => item.id === prev)) renderSelect.value = prev;
    }
    const attachBtn = document.getElementById('vision-render-attach');
    if (attachBtn) attachBtn.disabled = !state.visionRenderImage;
}

/** Reset studio to upload state */
function resetStudio() {
    state.visionRemix = { previewURL: '', analysis: null, file: null, status: '', storageURL: '', storageKey: '' };
    state.visionRenderImage = '';
    state.visionRenderAsset = null;
    state.visionDesign = null;
    document.getElementById('studio-upload-area')?.classList.remove('hidden');
    document.getElementById('studio-result')?.classList.add('hidden');
    const fileInput = document.getElementById('studio-file-input');
    if (fileInput) fileInput.value = '';
    setStudioStep(1);
}

async function attachRenderToListing() {
    const select = document.getElementById('vision-render-listing');
    const statusEl = document.getElementById('vision-render-link-status');
    if (!select || !statusEl) return;
    if (!state.visionRenderImage) {
        statusEl.textContent = 'Generera först en bild.';
        statusEl.classList.add('error');
        return;
    }
    const listingId = select.value;
    if (!listingId) {
        statusEl.textContent = 'Välj vilket objekt bilden ska kopplas till.';
        statusEl.classList.add('error');
        return;
    }
    statusEl.textContent = 'Kopplar bilden...';
    statusEl.classList.remove('error');
    try {
        let asset = state.visionRenderAsset;
        if (!asset || !asset.url) {
            const file = await dataURLToFile(state.visionRenderImage, `vision-${Date.now()}.png`);
            asset = await uploadMediaFile(file);
        }
        await attachImageToListing(listingId, {
            url: asset.url,
            key: asset.key,
            source: 'ai',
            kind: 'render',
            label: 'AI-rendering',
        });
        statusEl.textContent = 'Bilden kopplades till objektet.';
        await fetchListings();
    } catch (err) {
        statusEl.textContent = err.message;
        statusEl.classList.add('error');
    }
}

async function dataURLToFile(dataUrl, filename) {
    const res = await fetch(dataUrl);
    const blob = await res.blob();
    return new File([blob], filename, { type: blob.type || 'image/png' });
}

// Keep renderVisionLab as a no-op so any existing callers don't break
function renderVisionLab() {
    renderStudioResult();
}

async function attachImageToListing(id, asset) {
    const res = await fetch(`/api/listings/${id}/images`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(asset),
    });
    if (!res.ok) {
        const text = await res.text();
        throw new Error(text || 'Misslyckades med att koppla bild');
    }
    const listing = await res.json();
    const idx = state.listings.findIndex(item => item.id === listing.id);
    if (idx !== -1) {
        state.listings[idx] = listing;
    } else {
        state.listings.unshift(listing);
    }
    if (state.current && state.current.id === listing.id) {
        state.current = listing;
        renderDetail();
    }
    renderObjectList();
}

function getSelectionRange() {
    const editor = document.getElementById('full-editor');
    return { start: editor.selectionStart, end: editor.selectionEnd };
}

async function applySelectionRewrite(mode) {
    if (!state.current) return;
    const editor = document.getElementById('full-editor');
    const { start, end } = getSelectionRange();
    const selected = editor.value.slice(start, end) || editor.value;
    if (!selected.trim()) return;

    const instruction = instructionForMode(mode, state.current.tone);
    setAIStatus(`Omskriver: ${instruction}`, true);
    try {
        const targetSlug = getPrimarySectionSlug();
        const res = await fetch(`/api/listings/${state.selectedId}/sections/${targetSlug}/rewrite`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ instruction, selection: selected }),
        });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || 'Misslyckades med omskrivning');
        }
        state.current = await res.json();
        pushVersion(getFullCopy(state.current), 'Omskriven');
        renderDetail();
        incrementRewriteStat();
        setAIStatus('Omskriven klar.', false, true);
    } catch (err) {
        alert(err.message);
        setAIStatus('', false, true);
    }
}

function instructionForMode(mode, tone) {
    switch (mode) {
    case 'sales':
        return 'Gör texten mer säljande och varm utan att hitta på fakta.';
    case 'shorter':
        return 'Korta ned texten till det viktigaste men behåll flyt.';
    case 'simpler':
        return 'Skriv om till enklare språk och tydliga meningar.';
    case 'luxury':
        return 'Ge texten en mer lyxig och sofistikerad ton.';
    case 'longer':
        return 'Bygg ut texten med mer miljö och känsla utan nya fakta.';
    case 'rewrite':
    default:
        return `Skriv om med samma fakta i en varierad ${tone || 'neutral'} ton.`;
    }
}

async function regenerateWithTone() {
    if (!state.current) return;
    await applySelectionRewrite('rewrite');
}

function normalizeSlug(value) {
    let slug = (value || '').toString().trim().toLowerCase();
    if (!slug) return '';
    slug = slug.replace(/^\/+|\/+$/g, '');
    slug = slug.replace(/_/g, '-');
    slug = slug
        .split(/[\s]+/)
        .filter(Boolean)
        .join('-');
    slug = slug.replace(/^-+|-+$/g, '');
    return slug;
}

function getPrimarySectionSlug() {
    const sections = state.current?.sections || [];
    for (const section of sections) {
        const slug = normalizeSlug(section.slug || section.title);
        if (slug) return slug;
    }
    return 'main';
}

function bindEvents() {
    const form = document.getElementById('listing-form');
    form.addEventListener('submit', handleCreate);
    document.getElementById('property-type').addEventListener('change', toggleFloorField);
    document.getElementById('auth-toggle-login')?.addEventListener('click', () => setAuthMode('login'));
    document.getElementById('auth-toggle-register')?.addEventListener('click', () => setAuthMode('register'));
    document.getElementById('login-form')?.addEventListener('submit', handleLoginSubmit);
    document.getElementById('register-form')?.addEventListener('submit', handleRegisterSubmit);
    document.getElementById('logout-btn')?.addEventListener('click', handleLogout);
    document.querySelectorAll('[data-auth-trigger]').forEach(btn => {
        btn.addEventListener('click', () => {
            const mode = btn.dataset.authTrigger === 'register' ? 'register' : 'login';
            setAuthMode(mode);
            showAuthOverlay(authCopyForMode(mode));
        });
    });

    document.querySelectorAll('.selection-action').forEach(btn => {
        btn.addEventListener('click', () => applySelectionRewrite(btn.dataset.mode));
    });
    document.getElementById('regenerate-btn').addEventListener('click', regenerateWithTone);
    document.getElementById('copy-text-btn').addEventListener('click', copyFullText);
    document.getElementById('download-txt-btn').addEventListener('click', downloadText);
    document.getElementById('upload-btn').addEventListener('click', () => document.getElementById('file-input').click());
    document.getElementById('file-input').addEventListener('change', async (e) => {
        await handleFiles(e);
        e.target.value = '';
    });
    const visionRenderAttachBtn = document.getElementById('vision-render-attach');
    if (visionRenderAttachBtn) {
        visionRenderAttachBtn.addEventListener('click', attachRenderToListing);
    }

    // Studio (simplified image redesign) event bindings
    const studioFileInput = document.getElementById('studio-file-input');
    if (studioFileInput) {
        studioFileInput.addEventListener('change', handleStudioFileSelect);
    }
    const studioDropzone = document.getElementById('studio-dropzone');
    if (studioDropzone) {
        studioDropzone.addEventListener('dragover', e => { e.preventDefault(); studioDropzone.classList.add('dragging'); });
        studioDropzone.addEventListener('dragleave', () => studioDropzone.classList.remove('dragging'));
        studioDropzone.addEventListener('drop', e => {
            e.preventDefault();
            studioDropzone.classList.remove('dragging');
            const file = e.dataTransfer?.files?.[0];
            if (file) processStudioFile(file);
        });
    }
    document.getElementById('studio-generate-btn')?.addEventListener('click', handleStudioGenerate);
    document.getElementById('studio-dialog-close')?.addEventListener('click', closeStudioDialog);
    document.getElementById('studio-dialog-cancel')?.addEventListener('click', closeStudioDialog);
    document.getElementById('studio-redesign-btn')?.addEventListener('click', openStudioDialog);
    document.getElementById('studio-new-image-btn')?.addEventListener('click', resetStudio);
    const studioStyleGrid = document.getElementById('studio-style-grid');
    if (studioStyleGrid) {
        studioStyleGrid.addEventListener('click', handleStyleCardClick);
    }
    const studioOverlay = document.getElementById('studio-dialog-overlay');
    if (studioOverlay) {
        studioOverlay.addEventListener('click', e => { if (e.target === studioOverlay) closeStudioDialog(); });
    }

    const dropzone = document.getElementById('dropzone');
    dropzone.addEventListener('click', () => document.getElementById('file-input').click());
    dropzone.addEventListener('dragover', e => { e.preventDefault(); dropzone.classList.add('dragging'); });
    dropzone.addEventListener('dragleave', () => dropzone.classList.remove('dragging'));
    dropzone.addEventListener('drop', async e => {
        e.preventDefault();
        dropzone.classList.remove('dragging');
        await handleFiles({ target: { files: e.dataTransfer.files } });
    });

    const annualInput = document.getElementById('annual-file');
    const annualDrop = document.getElementById('annual-drop');
    if (annualInput) {
        annualInput.addEventListener('change', async e => {
            await handleAnnualFileChange(e.target.files);
            e.target.value = '';
        });
    }
    if (annualDrop) {
        annualDrop.addEventListener('click', () => annualInput?.click());
        annualDrop.addEventListener('dragover', e => { e.preventDefault(); annualDrop.classList.add('dragging'); });
        annualDrop.addEventListener('dragleave', () => annualDrop.classList.remove('dragging'));
        annualDrop.addEventListener('drop', e => {
            e.preventDefault();
            annualDrop.classList.remove('dragging');
            handleAnnualFileChange(e.dataTransfer?.files);
        });
    }

    // BRF Intel file upload listeners
    const brfIntelInput = document.getElementById('brfintel-file');
    const brfIntelDrop = document.getElementById('brfintel-drop');
    if (brfIntelInput) {
        brfIntelInput.addEventListener('change', async e => {
            handleBRFIntelFileChange(e.target.files);
            e.target.value = '';
        });
    }
    if (brfIntelDrop) {
        brfIntelDrop.addEventListener('click', () => brfIntelInput?.click());
        brfIntelDrop.addEventListener('dragover', e => { e.preventDefault(); brfIntelDrop.classList.add('dragging'); });
        brfIntelDrop.addEventListener('dragleave', () => brfIntelDrop.classList.remove('dragging'));
        brfIntelDrop.addEventListener('drop', e => {
            e.preventDefault();
            brfIntelDrop.classList.remove('dragging');
            handleBRFIntelFileChange(e.dataTransfer?.files);
        });
    }

    document.getElementById('clear-versions').addEventListener('click', () => {
        state.versions = [];
        renderVersions();
    });
    const profileForm = document.getElementById('profile-form');
    if (profileForm) {
        profileForm.addEventListener('submit', handleProfileFormSubmit);
    }
    const profileReset = document.getElementById('profile-reset');
    if (profileReset) {
        profileReset.addEventListener('click', resetProfileForm);
    }
    const styleSelect = document.getElementById('style-profile');
    if (styleSelect) {
        styleSelect.addEventListener('change', () => {
            state.selectedProfileId = styleSelect.value || '';
        });
    }
    const templateSelect = document.getElementById('template-select');
    if (templateSelect) {
        templateSelect.addEventListener('change', () => {
            state.selectedTemplateId = templateSelect.value || '';
        });
    }
    // Template form
    const templateForm = document.getElementById('template-form');
    if (templateForm) {
        templateForm.addEventListener('submit', handleTemplateFormSubmit);
    }
    const templateReset = document.getElementById('template-reset');
    if (templateReset) {
        templateReset.addEventListener('click', resetTemplateForm);
    }
    const templateAddSection = document.getElementById('template-add-section');
    if (templateAddSection) {
        templateAddSection.addEventListener('click', () => addTemplateSectionRow());
    }

    const sidebarToggle = document.getElementById('sidebar-toggle');
    if (sidebarToggle) {
        sidebarToggle.addEventListener('click', toggleSidebar);
    }
    const modalOverlay = document.getElementById('modal-overlay');
    const modalClose = document.getElementById('modal-close');
    const modalCopy = document.getElementById('modal-copy');
    if (modalOverlay) {
        modalOverlay.addEventListener('click', (e) => {
            if (e.target === modalOverlay) closeModal();
        });
    }
    if (modalClose) {
        modalClose.addEventListener('click', closeModal);
    }
    if (modalCopy) {
        modalCopy.addEventListener('click', () => {
            const textarea = document.getElementById('modal-textarea');
            if (!textarea || !textarea.value) return;
            navigator.clipboard.writeText(textarea.value);
            modalCopy.classList.add('copied');
            modalCopy.textContent = 'Kopierat!';
            setTimeout(() => {
                modalCopy.classList.remove('copied');
                modalCopy.textContent = 'Kopiera text';
            }, 1500);
        });
    }
    document.querySelectorAll('[data-view]').forEach(link => {
        link.addEventListener('click', event => {
            event.preventDefault();
            showView(link.dataset.view);
        });
    });
    document.querySelectorAll('[data-view-trigger]').forEach(btn => {
        btn.addEventListener('click', () => {
            const view = btn.dataset.viewTrigger;
            showView(view);
            if (view === 'generator' && !state.usageLocked) {
                resetGeneratorForm();
            }
        });
    });
    const refreshObjects = document.getElementById('refresh-objects');
    if (refreshObjects) {
        refreshObjects.addEventListener('click', fetchListings);
    }
    const objectSearch = document.getElementById('object-search');
    if (objectSearch) {
        objectSearch.addEventListener('input', handleObjectSearch);
    }
    document.addEventListener('keydown', event => {
        if (event.key === 'Escape') {
            closeSidebar();
            const overlay = document.getElementById('auth-overlay');
            if (overlay && !overlay.classList.contains('hidden')) {
                closeAuthOverlay();
            }
        }
    });
    initLandingMobileMenu();
    setupAuthOverlayDismiss();
    initSidebarState();
}

function toggleFloorField() {
    const type = document.getElementById('property-type').value;
    const field = document.getElementById('floor-field');
    field.style.display = type === 'lägenhet' ? 'block' : 'none';
}

toggleFloorField();

function copyFullText() {
    const text = getFullCopy(state.current || {});
    if (!text) return;
    navigator.clipboard.writeText(text);
}

function downloadText() {
    const text = getFullCopy(state.current || {});
    if (!text) return;
    const blob = new Blob([text], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `${state.current.address || 'annons'}.txt`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
}

function pushVersion(text, label) {
    if (!text) return;
    const timestamp = new Date().toLocaleTimeString('sv-SE', { hour: '2-digit', minute: '2-digit' });
    state.versions.unshift({ label, timestamp, text });
    state.versions = state.versions.slice(0, 6);
    document.getElementById('clear-versions').disabled = state.versions.length === 0;
}

function renderVersions() {
    const list = document.getElementById('version-list');
    list.innerHTML = '';
    document.getElementById('clear-versions').disabled = state.versions.length === 0;

    state.versions.forEach((version, idx) => {
        const item = document.createElement('div');
        item.className = 'version-item';
        const info = document.createElement('div');
        info.innerHTML = `<strong>${version.label}</strong> · ${version.timestamp}`;
        const actions = document.createElement('div');
        const restore = document.createElement('button');
        restore.textContent = 'Återställ';
        restore.addEventListener('click', () => applyVersion(idx));
        actions.appendChild(restore);
        item.appendChild(info);
        item.appendChild(actions);
        list.appendChild(item);
    });
}

function renderObjectList() {
    const container = document.getElementById('object-list');
    if (!container) return;
    container.innerHTML = '';
    const query = (state.listingFilter || '').toLowerCase();
    const entries = state.listings.filter(item => {
        if (!query) return true;
        const haystack = `${item.address || ''} ${item.city || ''} ${item.neighborhood || ''}`.toLowerCase();
        return haystack.includes(query);
    });

    if (!entries.length) {
        const empty = document.createElement('p');
        empty.className = 'empty-state';
        empty.textContent = state.listings.length
            ? 'Inga objekt matchar din sökning.'
            : 'Inga objekt ännu. Skapa din första annons.';
        container.appendChild(empty);
        return;
    }

    entries.forEach(listing => {
        const card = document.createElement('div');
        card.className = 'object-card';
        if (listing.id === state.selectedId) {
            card.classList.add('active');
        }

        const headerWrap = document.createElement('div');
        headerWrap.className = 'object-card__header';
        const thumb = document.createElement('div');
        thumb.className = 'object-card__thumb';
        if (listing.image_url) {
            thumb.style.backgroundImage = `url(${listing.image_url})`;
        }

        const title = document.createElement('div');
        title.className = 'object-card__title';
        title.textContent = listing.address || 'Namnlöst objekt';

        const meta = document.createElement('div');
        meta.className = 'object-card__meta';
        meta.textContent = buildListingMeta(listing);

        const status = document.createElement('div');
        status.className = 'object-card__status';
        status.textContent = buildListingStatus(listing);

        const body = document.createElement('div');
        body.className = 'object-card__body';
        body.appendChild(title);
        body.appendChild(meta);
        body.appendChild(status);
        headerWrap.appendChild(thumb);
        headerWrap.appendChild(body);
        card.appendChild(headerWrap);

        const actions = document.createElement('div');
        actions.className = 'object-card__actions';
        const openBtn = document.createElement('button');
        openBtn.type = 'button';
        openBtn.className = 'secondary';
        openBtn.textContent = 'Öppna';
        openBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            openListingModal(listing.id);
        });

        const editBtn = document.createElement('button');
        editBtn.type = 'button';
        editBtn.className = 'edit-btn';
        editBtn.textContent = 'Redigera';
        editBtn.addEventListener('click', async (e) => {
            e.stopPropagation();
            await startEditListing(listing.id);
        });

        const deleteBtn = document.createElement('button');
        deleteBtn.type = 'button';
        deleteBtn.className = 'ghost';
        deleteBtn.textContent = 'Ta bort';
        deleteBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            deleteListing(listing.id);
        });
        actions.appendChild(openBtn);
        actions.appendChild(editBtn);
        actions.appendChild(deleteBtn);

        card.appendChild(title);
        card.appendChild(meta);
        card.appendChild(status);
        card.appendChild(actions);
        card.addEventListener('click', () => selectListing(listing.id));
        container.appendChild(card);
    });
}

function buildListingMeta(listing) {
    const rooms = formatRoomsValue(Number(listing.rooms || 0));
    const areaValue = Number(listing.living_area);
    const area = Number.isFinite(areaValue) && areaValue > 0 ? `${Math.round(areaValue)} kvm` : '';
    const location = [listing.neighborhood, listing.city].filter(Boolean).join(', ');
    const homeFacts = rooms && area ? `${rooms} · ${area}` : rooms || area;
    return [homeFacts, location].filter(Boolean).join(' • ') || 'Detaljer saknas';
}

function buildListingStatus(listing) {
    const tone = capitalize(listing.tone) || 'Neutral ton';
    const length = capitalize(listing.length) || 'Normal längd';
    return `${tone} • ${length}`;
}

function formatRoomsValue(value) {
    if (!Number.isFinite(value) || value <= 0) {
        return '';
    }
    return Number.isInteger(value) ? `${value} rum` : `${value.toFixed(1)} rum`;
}

function populateFormFromListing(listing) {
    if (!listing) return;
    const setValue = (id, val) => {
        const el = document.getElementById(id);
        if (!el) return;
        if (el.type === 'checkbox') {
            el.checked = Boolean(val);
        } else {
            el.value = val ?? '';
        }
    };

    setValue('address', listing.address || '');
    setValue('neighborhood', listing.neighborhood || '');
    setValue('city', listing.city || '');
    setValue('property-type', listing.property_type || listing.propertyType || '');
    setValue('rooms', listing.rooms ?? '');
    setValue('living-area', listing.living_area ?? listing.livingArea ?? '');
    setValue('floor', listing.floor ?? '');
    setValue('condition', listing.condition ?? '');
    setValue('association', listing.association ?? '');
    setValue('balcony', listing.balcony);
    setValue('tone', listing.tone || 'Neutral');
    setValue('length', listing.length || 'normal');
    setValue('style-profile', listing.details?.meta?.style_profile_id || '');
    const styleSelect = document.getElementById('style-profile');
    if (styleSelect) {
        state.selectedProfileId = styleSelect.value || '';
    }

    if (Array.isArray(listing.highlights)) {
        setValue('highlights', listing.highlights.join(', '));
    } else if (listing.highlights) {
        setValue('highlights', listing.highlights);
    } else {
        setValue('highlights', '');
    }
    toggleFloorField();
}

function renderStyleProfileOptions() {
    const select = document.getElementById('style-profile');
    if (!select) return;
    const previous = select.value || state.selectedProfileId || '';
    select.innerHTML = '<option value="">Standard (ingen sparad ton)</option>';
    state.styleProfiles.forEach(profile => {
        const option = document.createElement('option');
        option.value = profile.id;
        option.textContent = profile.name;
        select.appendChild(option);
    });
    if (previous && state.styleProfiles.some(profile => profile.id === previous)) {
        select.value = previous;
    } else {
        select.value = '';
    }
    state.selectedProfileId = select.value || '';
}

function renderStyleProfileList() {
    const container = document.getElementById('profile-list');
    if (!container) return;
    container.innerHTML = '';
    if (!state.styleProfiles.length) {
        container.innerHTML = '<p class="muted">Inga sparade stilprofiler ännu.</p>';
        return;
    }
    state.styleProfiles.forEach(profile => {
        const card = document.createElement('div');
        card.className = 'profile-card';
        const header = document.createElement('div');
        header.className = 'profile-card__header';
        header.innerHTML = `<strong>${profile.name}</strong><span class="muted">${profile.tone || 'Okänd ton'}</span>`;
        const desc = document.createElement('p');
        desc.className = 'profile-card__desc muted';
        desc.textContent = profile.description || 'Ingen beskrivning.';
        const examples = document.createElement('p');
        examples.className = 'profile-card__examples';
        const examplePreview = (profile.example_texts || []).slice(0, 2).join(' • ');
        examples.textContent = examplePreview ? `Exempel: ${examplePreview}` : 'Inga exempel tillagda.';
        const actions = document.createElement('div');
        actions.className = 'profile-card__actions';
        const useBtn = document.createElement('button');
        useBtn.type = 'button';
        useBtn.className = 'secondary';
        useBtn.textContent = 'Använd i generatorn';
        useBtn.addEventListener('click', () => selectProfileForForm(profile.id));
        const editBtn = document.createElement('button');
        editBtn.type = 'button';
        editBtn.className = 'ghost';
        editBtn.textContent = 'Redigera';
        editBtn.addEventListener('click', () => fillProfileForm(profile));
        actions.appendChild(useBtn);
        actions.appendChild(editBtn);
        card.appendChild(header);
        card.appendChild(desc);
        card.appendChild(examples);
        if (profile.guidelines) {
            const guidelines = document.createElement('p');
            guidelines.className = 'profile-card__examples';
            guidelines.textContent = `Riktlinjer: ${profile.guidelines}`;
            card.appendChild(guidelines);
        }
        if (profile.forbidden_words && profile.forbidden_words.length) {
            const forbid = document.createElement('p');
            forbid.className = 'profile-card__examples';
            forbid.textContent = `Undvik: ${profile.forbidden_words.join(', ')}`;
            card.appendChild(forbid);
        }
        card.appendChild(actions);
        container.appendChild(card);
    });
}

function fillProfileForm(profile) {
    if (!profile) return;
    document.getElementById('profile-id').value = profile.id || '';
    document.getElementById('profile-name').value = profile.name || '';
    document.getElementById('profile-tone').value = profile.tone || '';
    document.getElementById('profile-description').value = profile.description || '';
    document.getElementById('profile-guidelines').value = profile.guidelines || '';
    document.getElementById('profile-examples').value = (profile.example_texts || []).join('\n');
    document.getElementById('profile-forbidden').value = (profile.forbidden_words || []).join('\n');
    setProfileStatus(`Redigerar ${profile.name}`, false);
    window.scrollTo({ top: document.getElementById('profile-form').offsetTop - 40, behavior: 'smooth' });
}

function resetProfileForm() {
    document.getElementById('profile-id').value = '';
    document.getElementById('profile-name').value = '';
    document.getElementById('profile-tone').value = '';
    document.getElementById('profile-description').value = '';
    document.getElementById('profile-guidelines').value = '';
    document.getElementById('profile-examples').value = '';
    document.getElementById('profile-forbidden').value = '';
    setProfileStatus('', false);
}

function setProfileStatus(message, isError) {
    const el = document.getElementById('profile-status');
    if (!el) return;
    el.textContent = message || '';
    el.style.color = isError ? '#b42318' : 'var(--muted)';
}

async function handleProfileFormSubmit(event) {
    event.preventDefault();
    const payload = {
        id: document.getElementById('profile-id').value.trim(),
        name: document.getElementById('profile-name').value.trim(),
        tone: document.getElementById('profile-tone').value.trim(),
        description: document.getElementById('profile-description').value.trim(),
        guidelines: document.getElementById('profile-guidelines').value.trim(),
        example_texts: listFromLines(document.getElementById('profile-examples').value || ''),
        forbidden_words: listFromLines(document.getElementById('profile-forbidden').value || ''),
    };
    if (!payload.name) {
        setProfileStatus('Namn krävs.', true);
        return;
    }
    if (!payload.example_texts.length) {
        setProfileStatus('Lägg till minst ett exempel.', true);
        return;
    }
    try {
        const res = await fetch('/api/style-profiles/', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });
        if (!res.ok) {
            const txt = await res.text();
            throw new Error(txt || 'Kunde inte spara stilprofil');
        }
        await fetchStyleProfiles();
        resetProfileForm();
        setProfileStatus('Sparad!', false);
    } catch (err) {
        setProfileStatus(err.message, true);
    }
}

function selectProfileForForm(id) {
    const select = document.getElementById('style-profile');
    if (!select) return;
    select.value = id || '';
    state.selectedProfileId = select.value || '';
    setProfileStatus(id ? 'Profil vald i generatorn.' : '', false);
}

// ── Template CRUD ──

function renderTemplateSelectOptions() {
    const select = document.getElementById('template-select');
    if (!select) return;
    const previous = select.value || state.selectedTemplateId || '';
    select.innerHTML = '<option value="">Standard (inledning, rum, område, sammanfattning)</option>';
    state.templates.forEach(tpl => {
        const option = document.createElement('option');
        option.value = tpl.id;
        const sectionCount = (tpl.sections || []).length;
        option.textContent = `${tpl.name} (${sectionCount} sektioner)`;
        select.appendChild(option);
    });
    if (previous && state.templates.some(t => t.id === previous)) {
        select.value = previous;
    } else {
        select.value = '';
    }
    state.selectedTemplateId = select.value || '';
}

function renderTemplateList() {
    const container = document.getElementById('template-list');
    if (!container) return;
    container.innerHTML = '';
    if (!state.templates.length) {
        container.innerHTML = '<p class="muted">Inga mallar skapade ännu.</p>';
        return;
    }
    state.templates.forEach(tpl => {
        const card = document.createElement('div');
        card.className = 'template-card';

        const header = document.createElement('div');
        header.className = 'template-card__header';
        header.innerHTML = `<strong>${tpl.name}</strong><span class="muted">${(tpl.sections || []).length} sektioner</span>`;

        const desc = document.createElement('p');
        desc.className = 'template-card__desc muted';
        desc.textContent = tpl.description || 'Ingen beskrivning.';

        const sectionsPreview = document.createElement('div');
        sectionsPreview.className = 'template-card__sections';
        (tpl.sections || []).forEach(sec => {
            const badge = document.createElement('span');
            badge.className = 'template-section-badge';
            badge.textContent = sec.title;
            if (sec.description) {
                badge.title = sec.description;
            }
            sectionsPreview.appendChild(badge);
        });

        const actions = document.createElement('div');
        actions.className = 'template-card__actions';

        const useBtn = document.createElement('button');
        useBtn.type = 'button';
        useBtn.className = 'secondary';
        useBtn.textContent = 'Använd i generatorn';
        useBtn.addEventListener('click', () => {
            state.selectedTemplateId = tpl.id;
            const select = document.getElementById('template-select');
            if (select) select.value = tpl.id;
            setTemplateStatus('Mall vald i generatorn.', false);
            showView('generator');
        });

        const editBtn = document.createElement('button');
        editBtn.type = 'button';
        editBtn.className = 'ghost';
        editBtn.textContent = 'Redigera';
        editBtn.addEventListener('click', () => fillTemplateForm(tpl));

        const deleteBtn = document.createElement('button');
        deleteBtn.type = 'button';
        deleteBtn.className = 'ghost';
        deleteBtn.style.color = '#b42318';
        deleteBtn.textContent = 'Ta bort';
        deleteBtn.addEventListener('click', () => deleteTemplate(tpl.id));

        actions.appendChild(useBtn);
        actions.appendChild(editBtn);
        actions.appendChild(deleteBtn);

        card.appendChild(header);
        card.appendChild(desc);
        card.appendChild(sectionsPreview);
        card.appendChild(actions);
        container.appendChild(card);
    });
}

function addTemplateSectionRow(slug = '', title = '', description = '') {
    const container = document.getElementById('template-sections');
    if (!container) return;
    const row = document.createElement('div');
    row.className = 'template-section-row';
    row.innerHTML = `
        <div class="template-section-row__fields">
            <input type="text" class="tpl-sec-title" placeholder="Sektionsnamn (t.ex. Inledning)" value="${escapeHtml(title)}" required>
            <input type="text" class="tpl-sec-slug" placeholder="Slug (t.ex. intro)" value="${escapeHtml(slug)}">
            <textarea class="tpl-sec-desc" rows="2" placeholder="Instruktion till AI (t.ex. 'Skriv en säljande ingress med fokus på läge och ljus')">${escapeHtml(description)}</textarea>
        </div>
        <button type="button" class="ghost template-section-remove" title="Ta bort sektion">✕</button>
    `;
    row.querySelector('.template-section-remove').addEventListener('click', () => row.remove());
    container.appendChild(row);
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str || '';
    return div.innerHTML;
}

function collectTemplateSections() {
    const rows = document.querySelectorAll('.template-section-row');
    const sections = [];
    rows.forEach((row, i) => {
        const title = row.querySelector('.tpl-sec-title')?.value.trim() || '';
        const slug = row.querySelector('.tpl-sec-slug')?.value.trim() || `section-${i + 1}`;
        const description = row.querySelector('.tpl-sec-desc')?.value.trim() || '';
        if (title) {
            sections.push({ slug, title, description });
        }
    });
    return sections;
}

function fillTemplateForm(tpl) {
    if (!tpl) return;
    document.getElementById('template-id').value = tpl.id || '';
    document.getElementById('template-name').value = tpl.name || '';
    document.getElementById('template-description').value = tpl.description || '';
    const container = document.getElementById('template-sections');
    if (container) container.innerHTML = '';
    (tpl.sections || []).forEach(sec => {
        addTemplateSectionRow(sec.slug, sec.title, sec.description);
    });
    setTemplateStatus(`Redigerar "${tpl.name}"`, false);
    const formEl = document.getElementById('template-form');
    if (formEl) {
        formEl.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
}

function resetTemplateForm() {
    document.getElementById('template-id').value = '';
    document.getElementById('template-name').value = '';
    document.getElementById('template-description').value = '';
    const container = document.getElementById('template-sections');
    if (container) container.innerHTML = '';
    setTemplateStatus('', false);
}

function setTemplateStatus(message, isError) {
    const el = document.getElementById('template-status');
    if (!el) return;
    el.textContent = message || '';
    el.style.color = isError ? '#b42318' : 'var(--muted)';
}

async function handleTemplateFormSubmit(event) {
    event.preventDefault();
    const sections = collectTemplateSections();
    if (sections.length === 0) {
        setTemplateStatus('Lägg till minst en sektion.', true);
        return;
    }
    const payload = {
        id: document.getElementById('template-id').value.trim(),
        name: document.getElementById('template-name').value.trim(),
        description: document.getElementById('template-description').value.trim(),
        sections,
    };
    if (!payload.name) {
        setTemplateStatus('Mallnamn krävs.', true);
        return;
    }
    try {
        const res = await fetch('/api/templates/', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });
        if (!res.ok) {
            const txt = await res.text();
            throw new Error(txt || 'Kunde inte spara mall');
        }
        await fetchTemplates();
        resetTemplateForm();
        setTemplateStatus('Mall sparad!', false);
    } catch (err) {
        setTemplateStatus(err.message, true);
    }
}

async function deleteTemplate(id) {
    if (!id) return;
    const ok = window.confirm('Ta bort denna mall?');
    if (!ok) return;
    try {
        const res = await fetch(`/api/templates/${id}/`, { method: 'DELETE' });
        if (!res.ok) {
            const txt = await res.text();
            throw new Error(txt || 'Kunde inte ta bort mall');
        }
        if (state.selectedTemplateId === id) {
            state.selectedTemplateId = '';
            const select = document.getElementById('template-select');
            if (select) select.value = '';
        }
        await fetchTemplates();
    } catch (err) {
        alert(err.message);
    }
}

// ── Generation Loading Overlay ──

let generationTimerInterval = null;
let generationStartTime = null;

function showGenerationLoader() {
    const loader = document.getElementById('generation-loader');
    if (!loader) return;
    loader.classList.remove('hidden');
    // Reset steps
    document.querySelectorAll('.generation-step').forEach(el => {
        el.classList.remove('active', 'done');
    });
    // Start timer
    generationStartTime = Date.now();
    const timerEl = document.getElementById('generation-timer');
    if (timerEl) timerEl.textContent = '0:00';
    generationTimerInterval = setInterval(() => {
        const elapsed = Math.floor((Date.now() - generationStartTime) / 1000);
        const mins = Math.floor(elapsed / 60);
        const secs = String(elapsed % 60).padStart(2, '0');
        if (timerEl) timerEl.textContent = `${mins}:${secs}`;
    }, 1000);
    // Animate steps
    const stepData = document.getElementById('gen-step-data');
    const stepGeo = document.getElementById('gen-step-geo');
    const stepAI = document.getElementById('gen-step-ai');
    if (stepData) stepData.classList.add('active');
    setTimeout(() => {
        if (stepData) { stepData.classList.remove('active'); stepData.classList.add('done'); }
        if (stepGeo) stepGeo.classList.add('active');
    }, 1500);
    setTimeout(() => {
        if (stepGeo) { stepGeo.classList.remove('active'); stepGeo.classList.add('done'); }
        if (stepAI) stepAI.classList.add('active');
    }, 3000);
}

function completeGenerationLoader() {
    const stepAI = document.getElementById('gen-step-ai');
    const stepDone = document.getElementById('gen-step-done');
    if (stepAI) { stepAI.classList.remove('active'); stepAI.classList.add('done'); }
    if (stepDone) stepDone.classList.add('done');
    if (generationTimerInterval) {
        clearInterval(generationTimerInterval);
        generationTimerInterval = null;
    }
    setTimeout(() => {
        hideGenerationLoader();
    }, 1200);
}

function hideGenerationLoader() {
    const loader = document.getElementById('generation-loader');
    if (loader) loader.classList.add('hidden');
    if (generationTimerInterval) {
        clearInterval(generationTimerInterval);
        generationTimerInterval = null;
    }
}

function resetGeneratorForm() {
    const form = document.getElementById('listing-form');
    if (form) {
        form.reset();
    }
    const msg = document.getElementById('form-message');
    if (msg) {
        msg.textContent = '';
    }
    const fileInput = document.getElementById('file-input');
    if (fileInput) {
        fileInput.value = '';
    }
    state.current = null;
    state.lastText = '';
    state.versions = [];
    state.editingListingId = null;
    renderDetail();
    renderVersions();
    state.uploads = [];
    renderUploads();
    setAIStatus('', false);
    hideGenerationLoader();
    toggleFloorField();
    const styleSelect = document.getElementById('style-profile');
    if (styleSelect) {
        styleSelect.value = state.selectedProfileId || '';
    }
    const templateSelect = document.getElementById('template-select');
    if (templateSelect) {
        templateSelect.value = state.selectedTemplateId || '';
    }
}

function capitalize(value) {
    if (!value) return '';
    const str = String(value).trim();
    if (!str) return '';
    return str.charAt(0).toUpperCase() + str.slice(1);
}

function applyVersion(index) {
    const version = state.versions[index];
    if (!version || !state.current) return;
    const editor = document.getElementById('full-editor');
    editor.value = version.text;
    state.current.full_copy = version.text;
    renderDetail();
}

function incrementRewriteStat() {
    const el = document.getElementById('stat-rewrites');
    const current = parseInt(el.textContent || '0', 10);
    el.textContent = current + 1;
}

async function handleFiles(event) {
    const files = Array.from(event.target.files || event.dataTransfer?.files || []);
    if (!files.length) return;
    for (const file of files) {
        await queueUpload(file, state.editingListingId);
    }
    renderUploads();
}

async function queueUpload(file, targetListingId) {
    const entry = {
        name: file.name,
        size: file.size,
        status: 'Laddar upp...',
        kind: 'photo',
        source: 'user',
        attached: Boolean(targetListingId),
    };
    state.uploads.push(entry);
    renderUploads();
    try {
        const result = await uploadMediaFile(file);
        entry.url = result.url;
        entry.key = result.key;
        if (targetListingId) {
            entry.status = 'Kopplar till objekt...';
            await attachImageToListing(targetListingId, {
                url: result.url,
                key: result.key,
                source: 'user',
                kind: 'photo',
            });
            entry.status = 'Tillagd i objekt';
        } else {
            entry.status = 'Klar';
        }
    } catch (err) {
        entry.status = err.message || 'Fel vid uppladdning';
        entry.error = err.message;
    }
    renderUploads();
}

async function uploadMediaFile(file, timeoutMs = 30000) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    try {
        const formData = new FormData();
        formData.append('file', file);
        const res = await fetch('/api/uploads', {
            method: 'POST',
            body: formData,
            signal: controller.signal,
        });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || 'Misslyckades med uppladdning');
        }
        const payload = await res.json();
        const normalized = {
            url: payload.url || payload.URL || '',
            key: payload.key || payload.Key || '',
        };
        if (!normalized.url) {
            throw new Error('Uppladdningen saknar URL – kontrollera S3-konfigurationen.');
        }
        return normalized;
    } catch (err) {
        if (err.name === 'AbortError') {
            throw new Error('Uppladdningen tog för lång tid. Kontrollera nätverket eller försök igen.');
        }
        throw err;
    } finally {
        clearTimeout(timer);
    }
}

async function extractAnnualReport(file, timeoutMs = 360000) {
    if (!file) return;
    state.annualReport.status = 'Analyserar årsredovisningen...';
    renderAnnualResult();
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    try {
        const formData = new FormData();
        formData.append('file', file);
        const res = await fetch('/api/annual-reports/extract', {
            method: 'POST',
            body: formData,
            signal: controller.signal,
        });
        if (res.status === 401) {
            handleUnauthorized('Sessionen gick ut. Logga in igen.');
            return;
        }
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || 'Misslyckades med att extrahera årsredovisning');
        }
        const payload = await res.json();
        state.annualReport.result = payload;
        state.annualReport.status = 'Klar';
        state.annualReport.fileName = file.name;
    } catch (err) {
        state.annualReport.status = err.name === 'AbortError'
            ? 'Tog för lång tid. Försök igen eller använd en mindre PDF.'
            : (err.message || 'Ett fel uppstod');
        state.annualReport.result = null;
    } finally {
        clearTimeout(timer);
        renderAnnualResult();
    }
}

function handleAnnualFileChange(fileList) {
    const file = fileList?.[0];
    if (!file) return;
    const name = (file.name || '').toLowerCase();
    if (!file.type.includes('pdf') && !name.endsWith('.pdf')) {
        state.annualReport.status = 'Endast PDF-filer stöds.';
        state.annualReport.result = null;
        renderAnnualResult();
        return;
    }
    tryClientPdfExtraction(file).catch(() => extractAnnualReport(file));
}

function renderAnnualResult() {
    const statusEl = document.getElementById('annual-status');
    const resultEl = document.getElementById('annual-result');
    if (!resultEl) return;
    if (statusEl) statusEl.textContent = state.annualReport.status || '';
    if (!state.annualReport.result) {
        resultEl.innerHTML = '<p class="muted">Ingen årsredovisning analyserad ännu.</p>';
        return;
    }
    const r = state.annualReport.result;
    const bullets = [
        r.summary ? `<li>${r.summary}</li>` : '',
        r.fee_per_month ? `<li><strong>Avgift:</strong> ${r.fee_per_month}</li>` : '',
        r.debt_per_sqm ? `<li><strong>Skuld/kvm:</strong> ${r.debt_per_sqm}</li>` : '',
        r.total_debt ? `<li><strong>Totala skulder:</strong> ${r.total_debt}</li>` : '',
        r.planned_maintenance ? `<li><strong>Planerat underhåll:</strong> ${r.planned_maintenance}</li>` : '',
        r.notable_risks ? `<li><strong>Risker:</strong> ${r.notable_risks}</li>` : '',
        r.energy_class ? `<li><strong>Energiklass:</strong> ${r.energy_class}</li>` : '',
        r.energy_consumption ? `<li><strong>Energi:</strong> ${r.energy_consumption}</li>` : '',
    ].filter(Boolean).join('');

    const badges = [
        r.source_pages ? `<span class="annual-badge">${r.source_pages} sidor</span>` : '',
        r.characters_analysed ? `<span class="annual-badge">${r.characters_analysed} tecken</span>` : '',
        state.annualReport.fileName ? `<span class="annual-badge">${state.annualReport.fileName}</span>` : '',
    ].filter(Boolean).join('');

    const keyLines = [
        ['Org.nr', r.org_number],
        ['Fastighetsbeteckning', r.property_designation],
        ['Byggår', r.build_year],
        ['BOA', r.boa_total],
        ['LOA', r.loa_total],
        ['Skulder till kreditinstitut', r.debt_credit_total],
        ['Kassa & bank', r.cash_and_bank],
        ['Årets resultat', r.net_result],
        ['Räntekostnader', r.interest_costs],
        ['Avskrivningar', r.depreciation],
        ['Intäkter årsavgifter', r.fee_income],
        ['Intäkter lokaler', r.rental_income],
        ['Markägande', r.land_status],
        ['Avgäld utgång', r.land_lease_expiry],
    ].filter(([, val]) => val);

    const reno = [
        r.renovations_done ? `<li><strong>Utfört:</strong> ${r.renovations_done}</li>` : '',
        r.renovations_planned ? `<li><strong>Planerat:</strong> ${r.renovations_planned}</li>` : '',
    ].filter(Boolean).join('');

    const listings = Array.isArray(state.listings) ? state.listings : [];
    const selectedIds = Array.isArray(state.annualReport.selectedIds) ? state.annualReport.selectedIds : [];
    const options = listings.map(item => {
        const label = item.address || 'Namnlöst objekt';
        const linked = item?.details?.association?.annual_report ? ' • kopplad' : '';
        const selected = selectedIds.includes(item.id) ? ' selected' : '';
        return `<option value="${item.id}"${selected}>${label}${linked}</option>`;
    }).join('');
    const hasListings = listings.length > 0;
    const selectionStatus = state.annualReport.linkStatus
        ? `<span class="link-status">${state.annualReport.linkStatus}</span>`
        : '';
    const saveBlock = hasListings
        ? `<div class="annual-save">
                <div class="annual-save__left">
                    <p class="muted">Koppla till objekt:</p>
                    <select id="annual-listing-select" multiple size="4" aria-label="Välj objekt att koppla årsredovisning till">
                        ${options}
                    </select>
                </div>
                <div class="annual-save__right">
                    <button type="button" class="secondary" id="annual-save-btn">Spara koppling</button>
                    <button type="button" class="ghost" id="annual-unlink-btn">Koppla bort</button>
                    ${selectionStatus}
                </div>
            </div>`
        : `<p class="muted">Skapa eller ladda in objekt för att kunna koppla årsredovisningen.</p>`;

    resultEl.innerHTML = `
        <div class="annual-badges">${badges}</div>
        <div class="annual-grid">
            <div class="annual-card">
                <strong>Nycklar</strong>
                <ul>${bullets || '<li>Inga nyckeltal hittades.</li>'}</ul>
            </div>
            <div class="annual-card">
                <strong>Styrelsens kommentarer</strong>
                <p>${r.board_comments || '—'}</p>
            </div>
            <div class="annual-card">
                <strong>Association & ekonomi</strong>
                <ul>
                    ${keyLines.map(([k,v]) => `<li><strong>${k}:</strong> ${v}</li>`).join('') || '<li>—</li>'}
                </ul>
            </div>
            <div class="annual-card">
                <strong>Renoveringar</strong>
                <ul>${reno || '<li>—</li>'}</ul>
            </div>
        </div>
        ${saveBlock}
    `;

    if (hasListings) {
        const selectEl = document.getElementById('annual-listing-select');
        if (selectEl) {
            selectEl.addEventListener('change', () => {
                const selected = Array.from(selectEl.selectedOptions || []).map(opt => opt.value);
                state.annualReport.selectedIds = selected;
                state.annualReport.linkStatus = selected.length
                    ? `Valt ${selected.length} objekt.`
                    : 'Inget objekt valt.';
                renderAnnualResult();
            });
        }
        const saveBtn = document.getElementById('annual-save-btn');
        if (saveBtn) {
            saveBtn.addEventListener('click', () => saveAnnualReportToListings());
        }
        const unlinkBtn = document.getElementById('annual-unlink-btn');
        if (unlinkBtn) {
            unlinkBtn.addEventListener('click', () => unlinkAnnualReportFromListings());
        }
    }
}

function getSelectedListing() {
    const id = state.selectedId;
    if (!id) return null;
    if (state.current && state.current.id === id) return state.current;
    return state.listings.find(item => item.id === id) || null;
}

async function saveAnnualReportToListings() {
    const ids = Array.isArray(state.annualReport.selectedIds) ? state.annualReport.selectedIds : [];
    if (!ids.length || !state.annualReport.result) {
        state.annualReport.linkStatus = 'Välj minst ett objekt först.';
        renderAnnualResult();
        return;
    }
    state.annualReport.status = ids.length === 1
        ? 'Sparar sammanfattning till objektet...'
        : `Sparar sammanfattning till ${ids.length} objekt...`;
    renderAnnualResult();
    try {
        const results = await Promise.all(ids.map(async id => {
            const res = await fetch(`/api/listings/${id}/annual-report`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(state.annualReport.result),
            });
            if (!res.ok) {
                const txt = await res.text();
                throw new Error(txt || 'Kunde inte spara sammanfattningen');
            }
            return res.json();
        }));
        results.forEach(updated => {
            const idx = state.listings.findIndex(item => item.id === updated.id);
            if (idx !== -1) {
                state.listings[idx] = updated;
            } else {
                state.listings.unshift(updated);
            }
            if (state.current && state.current.id === updated.id) {
                state.current = updated;
            }
        });
        state.annualReport.status = 'Koppling sparad.';
        state.annualReport.linkStatus = `Kopplat till ${ids.length} objekt.`;
        renderDetail();
        renderObjectList();
        renderAnnualResult();
    } catch (err) {
        state.annualReport.status = err.message || 'Misslyckades med att spara sammanfattningen.';
        renderAnnualResult();
    }
}

async function unlinkAnnualReportFromListings() {
    const ids = Array.isArray(state.annualReport.selectedIds) ? state.annualReport.selectedIds : [];
    if (!ids.length) {
        state.annualReport.linkStatus = 'Välj minst ett objekt att koppla bort.';
        renderAnnualResult();
        return;
    }
    state.annualReport.status = ids.length === 1
        ? 'Kopplar bort årsredovisningen från objektet...'
        : `Kopplar bort årsredovisningen från ${ids.length} objekt...`;
    renderAnnualResult();
    try {
        const results = await Promise.all(ids.map(async id => {
            const res = await fetch(`/api/listings/${id}/annual-report`, { method: 'DELETE' });
            if (!res.ok) {
                const txt = await res.text();
                throw new Error(txt || 'Kunde inte koppla bort årsredovisningen');
            }
            return res.json();
        }));
        results.forEach(updated => {
            const idx = state.listings.findIndex(item => item.id === updated.id);
            if (idx !== -1) {
                state.listings[idx] = updated;
            } else {
                state.listings.unshift(updated);
            }
            if (state.current && state.current.id === updated.id) {
                state.current = updated;
            }
        });
        state.annualReport.status = 'Koppling borttagen.';
        state.annualReport.linkStatus = `Koppling borttagen för ${ids.length} objekt.`;
        renderDetail();
        renderObjectList();
        renderAnnualResult();
    } catch (err) {
        state.annualReport.status = err.message || 'Misslyckades med att koppla bort årsredovisningen.';
        renderAnnualResult();
    }
}

async function tryClientPdfExtraction(file) {
    if (!window.pdfjsLib) {
        state.annualReport.status = 'Laddar upp till servern...';
        renderAnnualResult();
        await extractAnnualReport(file);
        return;
    }
    state.annualReport.status = 'Läser PDF i browsern...';
    renderAnnualResult();

    const arrayBuffer = await file.arrayBuffer();
    const pdf = await pdfjsLib.getDocument({ data: arrayBuffer }).promise;
    const maxPages = Math.min(pdf.numPages, 30);
    let text = '';
    for (let i = 1; i <= maxPages; i++) {
        const page = await pdf.getPage(i);
        const content = await page.getTextContent();
        const strings = content.items.map(item => item.str).filter(Boolean);
        text += strings.join(' ') + '\n';
    }
    const sanitized = text.trim();
    if (!sanitized) {
        state.annualReport.status = 'Ingen text hittades i PDF:en, laddar upp till servern...';
        renderAnnualResult();
        await extractAnnualReport(file);
        return;
    }

    // Send text to summarize endpoint
    state.annualReport.status = 'Analyserar text lokalt...';
    renderAnnualResult();
    const payload = {
        text: sanitized.slice(0, 12000),
        file_name: file.name,
        pages: maxPages,
    };
    const res = await fetch('/api/annual-reports/summarize', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    if (res.status === 401) {
        handleUnauthorized('Sessionen gick ut. Logga in igen.');
        return;
    }
    if (!res.ok) {
        const txt = await res.text();
        throw new Error(txt || 'Misslyckades med att extrahera text');
    }
    const data = await res.json();
    state.annualReport.result = data;
    state.annualReport.status = 'Klar';
    state.annualReport.fileName = file.name;
    renderAnnualResult();
}


// ──────────────────────────────────────────────────────────────────────
// BRF Intelligence Engine — Frontend
// ──────────────────────────────────────────────────────────────────────

const brfIntelState = {
    status: '',
    report: null,
    history: [],
    uploading: false,
    timerInterval: null,
    timerStart: 0,
};

function startBRFIntelLoader() {
    const loader = document.getElementById('brfintel-loader');
    const timerEl = document.getElementById('brfintel-timer');
    const textEl = document.getElementById('brfintel-loader-text');
    const stepsEl = document.getElementById('brfintel-loader-steps');
    if (!loader) return;
    loader.classList.add('active');
    brfIntelState.timerStart = Date.now();

    const phases = [
        { after: 0,  label: 'Laddar upp PDF...' },
        { after: 3,  label: 'Extraherar text ur PDF...' },
        { after: 8,  label: 'Kontrollerar textkvalitet...' },
        { after: 14, label: 'AI-OCR pågår (skannad PDF)...' },
        { after: 30, label: 'Analyserar ekonomi och nyckeltal...' },
        { after: 50, label: 'Beräknar poäng och risker...' },
        { after: 70, label: 'Skriver köparsammanfattning...' },
        { after: 90, label: 'Nästan klar...' },
    ];

    function tick() {
        const elapsed = Math.floor((Date.now() - brfIntelState.timerStart) / 1000);
        const mins = Math.floor(elapsed / 60);
        const secs = elapsed % 60;
        if (timerEl) timerEl.textContent = `${mins}:${secs.toString().padStart(2, '0')}`;

        // Update phase text
        let currentPhase = phases[0];
        for (const p of phases) {
            if (elapsed >= p.after) currentPhase = p;
        }
        if (textEl) textEl.textContent = currentPhase.label;

        // Update step list
        if (stepsEl) {
            stepsEl.innerHTML = phases
                .filter(p => p.after <= elapsed + 2)
                .map(p => {
                    const done = elapsed >= p.after + 5;
                    const current = !done && elapsed >= p.after;
                    const cls = done ? 'done' : current ? 'current' : '';
                    return `<span class="brfintel-loader__step ${cls}">${p.label.replace('...', '')}</span>`;
                }).join('');
        }
    }
    tick();
    brfIntelState.timerInterval = setInterval(tick, 1000);
}

function stopBRFIntelLoader() {
    if (brfIntelState.timerInterval) {
        clearInterval(brfIntelState.timerInterval);
        brfIntelState.timerInterval = null;
    }
    const loader = document.getElementById('brfintel-loader');
    if (loader) loader.classList.remove('active');
}

function handleBRFIntelFileChange(fileList) {
    const file = fileList?.[0];
    if (!file) return;
    const name = (file.name || '').toLowerCase();
    if (!file.type.includes('pdf') && !name.endsWith('.pdf')) {
        brfIntelState.status = 'Endast PDF-filer stöds.';
        brfIntelState.report = null;
        renderBRFIntelResult();
        return;
    }
    uploadBRFIntelPDF(file);
}

async function uploadBRFIntelPDF(file) {
    if (brfIntelState.uploading) return;
    brfIntelState.uploading = true;
    brfIntelState.status = '';
    brfIntelState.report = null;
    renderBRFIntelResult();
    startBRFIntelLoader();

    const formData = new FormData();
    formData.append('file', file);
    formData.append('brf_name', document.getElementById('brfintel-name')?.value?.trim() || 'Okänd förening');
    formData.append('city', document.getElementById('brfintel-city')?.value?.trim() || '');
    formData.append('municipality', document.getElementById('brfintel-municipality')?.value?.trim() || '');

    try {
        const res = await fetch('/api/brf-intel/analyze-pdf', {
            method: 'POST',
            body: formData,
        });
        if (res.status === 401) {
            handleUnauthorized('Sessionen gick ut. Logga in igen.');
            return;
        }
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || 'Analysen misslyckades');
        }
        const data = await res.json();
        brfIntelState.report = data.report;
        brfIntelState.status = 'Analys klar!';
    } catch (err) {
        brfIntelState.status = err.message || 'Ett fel uppstod vid analysen.';
        brfIntelState.report = null;
    } finally {
        brfIntelState.uploading = false;
        stopBRFIntelLoader();
        renderBRFIntelResult();
        loadBRFIntelHistory();
    }
}

function renderBRFIntelResult() {
    const statusEl = document.getElementById('brfintel-status');
    const resultEl = document.getElementById('brfintel-result');
    if (!resultEl) return;
    if (statusEl) statusEl.textContent = brfIntelState.status || '';

    if (!brfIntelState.report) {
        resultEl.innerHTML = '<p class="muted">Ingen BRF-analys genomförd ännu.</p>';
        return;
    }

    const r = brfIntelState.report;
    const score = r.score || {};
    const dims = (score.dimensions || []).map(d =>
        `<div class="brfintel-dim">
            <div class="brfintel-dim__header">
                <span>${d.name}</span>
                <span class="brfintel-dim__score">${d.score}/100</span>
            </div>
            <div class="brfintel-bar">
                <div class="brfintel-bar__fill" style="width:${d.score}%; background:${barColor(d.score)}"></div>
            </div>
            <p class="muted small">${d.description || ''}</p>
        </div>`
    ).join('');

    const risks = (r.risks || []).map(risk => {
        const icon = {critical:'🔴',high:'🟠',medium:'🟡',low:'🟢'}[risk.severity] || 'ℹ️';
        return `<div class="brfintel-risk brfintel-risk--${risk.severity}">
            <span class="brfintel-risk__icon">${icon}</span>
            <div>
                <strong>${risk.title}</strong>
                <p class="muted small">${risk.description}</p>
                ${risk.metric ? `<span class="brfintel-risk__metric">${risk.metric}</span>` : ''}
            </div>
        </div>`;
    }).join('') || '<p class="muted">Inga riskvarningar.</p>';

    const trends = r.trends || {};
    const trendDirection = {improving:'📈 Förbättras', declining:'📉 Försämras', stable:'📊 Stabil', insufficient_data:'📋 Otillräcklig data'}[trends.direction] || trends.direction || '';
    const trendPoints = (trends.data_points || []).map(tp => {
        const metrics = [];
        if (tp.avgift_per_sqm) metrics.push(`Avgift/m²: ${Math.round(tp.avgift_per_sqm).toLocaleString('sv-SE')} kr`);
        if (tp.skuld_per_sqm) metrics.push(`Skuld/m²: ${Math.round(tp.skuld_per_sqm).toLocaleString('sv-SE')} kr`);
        if (tp.arsresultat) metrics.push(`Årsresultat: ${Math.round(tp.arsresultat).toLocaleString('sv-SE')} kr`);
        if (tp.likviditet) metrics.push(`Likviditet: ${Math.round(tp.likviditet).toLocaleString('sv-SE')} kr`);
        if (tp.reparationsfond) metrics.push(`Reparationsfond: ${Math.round(tp.reparationsfond).toLocaleString('sv-SE')} kr`);
        if (tp.rantekostnad) metrics.push(`Räntekostnad: ${Math.round(tp.rantekostnad).toLocaleString('sv-SE')} kr`);
        if (!metrics.length) return '';
        return `<li><strong>${tp.year || '—'}:</strong> ${metrics.join(' · ')}</li>`;
    }).filter(Boolean).join('');

    const fin = r.financials || {};
    const finRows = [
        ['Skuld/m²', fin.debt_per_sqm ? `${Math.round(fin.debt_per_sqm).toLocaleString('sv-SE')} kr` : '—'],
        ['Avgift/m²/år', fin.avgift_per_sqm ? `${Math.round(fin.avgift_per_sqm).toLocaleString('sv-SE')} kr` : '—'],
        ['Avgift/mån', fin.fee_per_month ? `${Math.round(fin.fee_per_month).toLocaleString('sv-SE')} kr` : '—'],
        ['Totala skulder', fin.total_debt ? `${Math.round(fin.total_debt).toLocaleString('sv-SE')} kr` : '—'],
        ['Kassa & bank', fin.cash_and_bank ? `${Math.round(fin.cash_and_bank).toLocaleString('sv-SE')} kr` : '—'],
        ['Reparationsfond', fin.repair_fund ? `${Math.round(fin.repair_fund).toLocaleString('sv-SE')} kr` : '—'],
        ['Räntekostnader', fin.interest_costs ? `${Math.round(fin.interest_costs).toLocaleString('sv-SE')} kr` : '—'],
        ['Årsresultat', fin.net_result ? `${Math.round(fin.net_result).toLocaleString('sv-SE')} kr` : '—'],
        ['Byggår', fin.build_year || '—'],
        ['Markstatus', fin.land_status || '—'],
        ['Energiklass', fin.energy_class || '—'],
    ].filter(([,v]) => v !== '—');

    const comparison = r.comparison;
    let comparisonHtml = '';
    if (comparison && comparison.metrics && comparison.metrics.length > 0) {
        const compRows = comparison.metrics.map(m =>
            `<tr>
                <td>${m.name}</td>
                <td>${Math.round(m.this_brf).toLocaleString('sv-SE')} ${m.unit}</td>
                <td>${Math.round(m.peer_median).toLocaleString('sv-SE')} ${m.unit}</td>
                <td>${m.better ? '✅' : '⚠️'}</td>
            </tr>`
        ).join('');
        comparisonHtml = `
            <div class="brfintel-card">
                <h4>📊 Jämförelse med liknande föreningar</h4>
                <p class="muted small">${comparison.peer_count || 0} jämförbara föreningar i ${comparison.peer_group || 'samma område'}</p>
                <table class="brfintel-table">
                    <thead><tr><th>Nyckeltal</th><th>Denna BRF</th><th>Median</th><th></th></tr></thead>
                    <tbody>${compRows}</tbody>
                </table>
            </div>`;
    }

    const sourceDocs = (r.source_documents || []).map(d =>
        `<span class="annual-badge">${d.file_name || 'PDF'} · ${d.page_count || '?'} sidor · ${d.char_count || '?'} tecken</span>`
    ).join('');

    resultEl.innerHTML = `
        <div class="brfintel-score-hero">
            <div class="brfintel-score-circle brfintel-grade-${(score.grade || 'C').toLowerCase()}">
                <span class="brfintel-score-number">${score.total || 0}</span>
                <span class="brfintel-score-max">/100</span>
            </div>
            <div class="brfintel-score-meta">
                <span class="brfintel-grade-badge">${score.grade || '—'}</span>
                <span class="brfintel-score-label">${score.label || ''}</span>
                <span class="muted">${r.brf_name || ''} ${r.org_number ? '(' + r.org_number + ')' : ''}</span>
            </div>
        </div>

        <div class="brfintel-dims">${dims}</div>

        <div class="brfintel-grid">
            <div class="brfintel-card">
                <h4>⚠️ Riskvarningar</h4>
                ${risks}
            </div>

            <div class="brfintel-card">
                <h4>📈 Ekonomisk trend</h4>
                <p><strong>${trendDirection}</strong></p>
                <p class="muted">${trends.summary || ''}</p>
                ${trendPoints ? `<ul class="brfintel-trend-list">${trendPoints}</ul>` : ''}
            </div>

            <div class="brfintel-card">
                <h4>🔑 Nyckeltal</h4>
                <table class="brfintel-table">
                    <tbody>
                        ${finRows.map(([k,v]) => `<tr><td>${k}</td><td>${v}</td></tr>`).join('')}
                    </tbody>
                </table>
            </div>

            ${comparisonHtml}
        </div>

        ${r.buyer_summary ? `
        <div class="brfintel-card brfintel-card--full">
            <h4>🏠 Köparsammanfattning</h4>
            <div class="brfintel-prose">${formatProse(r.buyer_summary)}</div>
        </div>` : ''}

        ${r.legal_view ? `
        <div class="brfintel-card brfintel-card--full">
            <h4>⚖️ Juridisk säkerhetsvy (mäklare)</h4>
            <div class="brfintel-prose">${formatProse(r.legal_view)}</div>
        </div>` : ''}

        ${sourceDocs ? `<div class="brfintel-sources">${sourceDocs}</div>` : ''}
    `;
}

function barColor(score) {
    if (score >= 75) return '#22c55e';
    if (score >= 50) return '#eab308';
    if (score >= 30) return '#f97316';
    return '#ef4444';
}

function formatProse(text) {
    if (!text) return '';
    return text.split(/\n{2,}/).map(p => `<p>${p.replace(/\n/g, '<br>')}</p>`).join('');
}

async function loadBRFIntelHistory() {
    const listEl = document.getElementById('brfintel-history-list');
    if (!listEl) return;
    try {
        const res = await fetch('/api/brf-intel/recent');
        if (!res.ok) return;
        const data = await res.json();
        brfIntelState.history = Array.isArray(data) ? data : [];
    } catch {
        brfIntelState.history = [];
    }
    renderBRFIntelHistory();
}

function renderBRFIntelHistory() {
    const listEl = document.getElementById('brfintel-history-list');
    if (!listEl) return;
    if (!brfIntelState.history.length) {
        listEl.innerHTML = '<p class="muted">Inga tidigare analyser.</p>';
        return;
    }
    listEl.innerHTML = brfIntelState.history.map(r => `
        <div class="brfintel-history-item" data-report-id="${r.id}">
            <div class="brfintel-history-score brfintel-grade-${(r.grade || 'c').toLowerCase()}">${r.score}</div>
            <div class="brfintel-history-meta">
                <strong>${r.brf_name}</strong>
                <span class="muted small">${new Date(r.created_at).toLocaleDateString('sv-SE')} · ${r.risk_count} varningar</span>
            </div>
            <button class="ghost small" onclick="loadBRFIntelReport('${r.id}')">Visa</button>
        </div>
    `).join('');
}

async function loadBRFIntelReport(id) {
    brfIntelState.status = 'Laddar rapport...';
    renderBRFIntelResult();
    try {
        const res = await fetch(`/api/brf-intel/reports/${id}`);
        if (res.status === 401) {
            handleUnauthorized('Sessionen gick ut. Logga in igen.');
            return;
        }
        if (!res.ok) throw new Error('Kunde inte hämta rapporten.');
        const report = await res.json();
        brfIntelState.report = report;
        brfIntelState.status = '';
    } catch (err) {
        brfIntelState.status = err.message || 'Ett fel uppstod.';
    }
    renderBRFIntelResult();
}


function renderUploads() {
    const list = document.getElementById('upload-list');
    if (!list) return;
    list.innerHTML = '';
    state.uploads.forEach(file => {
        const item = document.createElement('div');
        item.className = 'upload-item';
        const name = document.createElement('span');
        name.textContent = file.name;
        const status = document.createElement('span');
        status.className = 'upload-item__status';
        status.textContent = file.status;
        item.appendChild(name);
        item.appendChild(status);
        list.appendChild(item);
    });
    updateImageStats();
}

function setAIStatus(message, busy, hideLater) {
    const el = document.getElementById('ai-status');
    if (!message) {
        el.classList.add('hidden');
        return;
    }
    el.textContent = message;
    el.classList.remove('hidden');
    if (busy) {
        el.classList.add('pulse');
    } else {
        el.classList.remove('pulse');
    }
    if (hideLater) {
        setTimeout(() => el.classList.add('hidden'), 2200);
    }
}

function handleObjectSearch(event) {
    state.listingFilter = event.target.value.toLowerCase();
    renderObjectList();
}

async function startEditListing(id) {
    if (!id) return;
    await selectListing(id);
    const detail = state.current || state.listings.find(item => item.id === id);
    if (!detail) return;
    state.editingListingId = id;
    populateFormFromListing(detail);
    state.uploads = [];
    renderUploads();
    showView('generator');
    document.getElementById('listing-form')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

function updateVolumeStats() {
    const totalEl = document.getElementById('stat-total');
    const weekEl = document.getElementById('stat-week');
    const monthEl = document.getElementById('stat-month');

    const listings = Array.isArray(state.listings) ? state.listings : [];
    const total = listings.length;
    const week = countListingsWithinDays(7);
    const month = countListingsWithinDays(30);

    if (totalEl) totalEl.textContent = total;
    if (weekEl) weekEl.textContent = week;
    if (monthEl) monthEl.textContent = month;
}

function updateTimeSavings() {
    const assumedManualMinutes = 45; // uppskattad manuell tid per annons
    const assumedAIEditableMinutes = 10; // uppskattad tid med AI + justering
    const savedPerAd = Math.max(assumedManualMinutes - assumedAIEditableMinutes, 0);

    const now = new Date();
    const msInDay = 86400000;
    const listings = state.listings || [];
    const recent = listings.filter(item => {
        if (!item.created_at) return true;
        const created = new Date(item.created_at);
        return Number.isFinite(created.getTime()) && (now - created) <= 30 * msInDay;
    });

    const savedMonthly = savedPerAd * recent.length;
    const savedTotal = savedPerAd * listings.length;

    const avgEl = document.getElementById('stat-saved-avg');
    const monthEl = document.getElementById('stat-saved-month');
    const totalEl = document.getElementById('stat-saved-total');

    if (avgEl) avgEl.textContent = formatMinutes(savedPerAd);
    if (monthEl) monthEl.textContent = formatMinutes(savedMonthly);
    if (totalEl) totalEl.textContent = formatMinutes(savedTotal);
}

function countListingsWithinDays(days) {
    const now = new Date();
    const limit = days * 86400000;
    return (state.listings || []).filter(item => {
        if (!item.created_at) return true;
        const created = new Date(item.created_at);
        if (!Number.isFinite(created.getTime())) return false;
        return (now - created) <= limit;
    }).length;
}

function formatMinutes(minutes) {
    const mins = Math.max(0, Math.round(minutes));
    if (mins < 90) return `${mins} min`;
    const hours = Math.floor(mins / 60);
    const rem = mins % 60;
    return rem ? `${hours} h ${rem} min` : `${hours} h`;
}

function showView(view) {
    if (state.usageLocked && view !== 'settings') {
        view = 'settings';
    }
    const targetId = `view-${view}`;
    document.body.className = document.body.className
        .split(' ')
        .filter(cls => !cls.startsWith('view-'))
        .concat(`view-${view}`)
        .join(' ');

    document.querySelectorAll('.view').forEach(el => {
        el.classList.toggle('view--active', el.id === targetId);
    });
    document.querySelectorAll('[data-view]').forEach(link => {
        link.classList.toggle('active', link.dataset.view === view);
    });
    updateTopbarCopy(view);
    if (view === 'vision') {
        renderVisionLab();
    }
    if (view === 'templates') {
        fetchTemplates();
    }
    if (view === 'style-profiles') {
        fetchStyleProfiles();
    }
    if (view === 'brfintel') {
        loadBRFIntelHistory();
    }
    if (window.innerWidth < 900) {
        closeSidebar();
    }
}

function updateTopbarCopy(view) {
    const titleEl = document.getElementById('topbar-title');
    const subtitleEl = document.getElementById('topbar-subtitle');
    const copy = {
        generator: {
            title: 'Annonsgenerator',
            subtitle: 'Skapa och omskriv annonser.',
        },
        objects: {
            title: 'Mina objekt',
            subtitle: 'Hantera och öppna befintliga annonser.',
        },
        stats: {
            title: 'Statistik',
            subtitle: 'Överblick över aktivitet och omskrivningar.',
        },
        vision: {
            title: 'Bildstudio',
            subtitle: 'Analysera bilder och skapa designförslag.',
        },
        annuals: {
            title: 'Extrahera årsredovisningar',
            subtitle: 'Plocka ut nyckeltal ur BRF-PDF.',
        },
        brfintel: {
            title: 'BRF Analys',
            subtitle: 'Djupanalys med poäng, risker och trender.',
        },
        images: {
            title: 'Bildhantering',
            subtitle: 'Hantera och ladda upp bildmaterial.',
        },
        templates: {
            title: 'Mallar',
            subtitle: 'Återanvänd strukturer och tonlägen.',
        },
        'style-profiles': {
            title: 'Träna tonen',
            subtitle: 'Skapa och hantera stilprofiler för varje kund.',
        },
        settings: {
            title: 'Inställningar',
            subtitle: 'Kontroll över konto, team och integrationer.',
        },
    }[view] || { title: 'Broker AI', subtitle: '' };

    if (titleEl) titleEl.textContent = copy.title;
    if (subtitleEl) subtitleEl.textContent = copy.subtitle || '';
}

function toggleSidebar() {
    document.body.classList.toggle('sidebar-open');
    updateSidebarToggleState();
}

function closeSidebar() {
    if (!document.body.classList.contains('sidebar-open')) {
        return;
    }
    document.body.classList.remove('sidebar-open');
    updateSidebarToggleState();
}

function initSidebarState() {
    if (window.innerWidth < 900) {
        document.body.classList.remove('sidebar-open');
    }
    updateSidebarToggleState();
}

function updateSidebarToggleState() {
    const toggle = document.getElementById('sidebar-toggle');
    if (!toggle) return;
    const open = document.body.classList.contains('sidebar-open');
    toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
    toggle.setAttribute('aria-label', open ? 'Faell ihop meny' : 'Visa meny');
    const icon = toggle.querySelector('span');
    if (icon) {
        icon.textContent = open ? 'X' : '≡';
    }
}

function updateImageStats() {
    const processed = state.uploads.filter(file => file.url).length;
    const el = document.getElementById('stat-images');
    const avgEl = document.getElementById('stat-images-avg');
    const listingCount = state.listings.length;
    const average = listingCount ? processed / listingCount : 0;

    if (el) el.textContent = processed;
    if (avgEl) avgEl.textContent = average.toFixed(1);
}

bindEvents();
renderVisionLab();
renderAnnualResult();
showView('objects');
checkSession().finally(maybeOpenAuthFromQuery);

async function initApp() {
    if (!state.user) return;
    await fetchStyleProfiles();
    await fetchTemplates();
    await fetchListings();
    renderSubscriptionStatus();
}

// ── Stripe Billing ──

function renderSubscriptionStatus() {
    const infoEl = document.getElementById('subscription-info');
    const actionsEl = document.getElementById('subscription-actions');
    if (!infoEl || !actionsEl) return;

    const user = state.user;
    if (!user) {
        infoEl.innerHTML = '<p class="muted">Logga in för att se prenumeration.</p>';
        actionsEl.innerHTML = '';
        return;
    }

    const status = user.subscription_status || '';
    const isPaid = status === 'active' || status === 'trialing';
    const usageCount = user.usage_count || 0;
    const usageLimit = isPaid ? (user.usage_limit || 3) : 3;
    let badge = '';
    let description = '';

    switch (status) {
        case 'active':
            badge = '<span class="badge badge--success">Aktiv</span>';
            description = 'Din prenumeration är aktiv. Du har obegränsad tillgång till alla AI-funktioner.';
            break;
        case 'trialing':
            badge = '<span class="badge badge--info">Provperiod</span>';
            description = 'Du använder en kostnadsfri provperiod med obegränsade anrop.';
            break;
        case 'past_due':
            badge = '<span class="badge badge--warning">Förfallen betalning</span>';
            description = 'Senaste betalningen misslyckades. Uppdatera din betalningsmetod för att fortsätta.';
            break;
        case 'canceled':
            badge = '<span class="badge badge--danger">Avslutad</span>';
            description = 'Din prenumeration har avslutats. Starta en ny för att fortsätta använda AI-funktionerna.';
            break;
        default:
            badge = '<span class="badge badge--neutral">Gratisplan</span>';
            description = `Du använder gratisplanen med ${usageLimit} kostnadsfria AI-anrop.`;
            break;
    }

    // Usage meter
    const remaining = isPaid ? '∞' : Math.max(0, usageLimit - usageCount);
    const usagePct = isPaid ? 0 : Math.min(100, Math.round((usageCount / usageLimit) * 100));
    const meterColor = usagePct >= 100 ? '#ef4444' : usagePct >= 70 ? '#eab308' : '#22c55e';

    infoEl.innerHTML = `
        <div style="display:flex;align-items:center;gap:0.75rem;margin-bottom:0.5rem;">
            <strong>Status:</strong> ${badge}
        </div>
        <p class="muted">${description}</p>
        <div class="usage-meter" style="margin-top:1rem;">
            <div style="display:flex;justify-content:space-between;margin-bottom:4px;">
                <span class="muted" style="font-size:0.85rem;">AI-anrop använda</span>
                <strong style="font-size:0.85rem;">${isPaid ? 'Obegränsat' : usageCount + ' / ' + usageLimit}</strong>
            </div>
            ${!isPaid ? `
            <div style="background:#e5e7eb;border-radius:6px;height:8px;overflow:hidden;">
                <div style="background:${meterColor};height:100%;width:${usagePct}%;border-radius:6px;transition:width 0.3s;"></div>
            </div>
            <p class="muted" style="font-size:0.8rem;margin-top:4px;">${remaining === 0 ? 'Inga anrop kvar – uppgradera för att fortsätta.' : remaining + ' anrop kvar'}</p>
            ` : ''}
        </div>
    `;

    let buttons = '';
    if (status === 'active' || status === 'trialing' || status === 'past_due') {
        buttons = '<button class="secondary" onclick="openBillingPortal()">Hantera prenumeration</button>';
    } else {
        buttons = '<div id="stripe-pricing-table-container"></div>';
    }
    actionsEl.innerHTML = buttons;

    // Embed Stripe Pricing Table if the user isn't paid.
    if (!isPaid) {
        renderStripePricingTable();
    }

    // Also update the sidebar mini-widget
    renderSidebarUsage();
}

function renderSidebarUsage() {
    const el = document.getElementById('sidebar-usage');
    if (!el) return;
    const user = state.user;
    if (!user) { el.innerHTML = ''; return; }

    const status = user.subscription_status || '';
    const isPaid = status === 'active' || status === 'trialing';
    const usageCount = user.usage_count || 0;
    const usageLimit = isPaid ? (user.usage_limit || 3) : 3;
    const remaining = isPaid ? null : Math.max(0, usageLimit - usageCount);
    const pct = isPaid ? 0 : Math.min(100, Math.round((usageCount / usageLimit) * 100));
    const color = pct >= 100 ? '#ef4444' : pct >= 70 ? '#eab308' : '#22c55e';

    if (isPaid) {
        el.innerHTML = `<span style="color:#22c55e;font-weight:600;">Pro – obegränsat</span>`;
    } else {
        el.innerHTML = `
            <div style="display:flex;justify-content:space-between;align-items:baseline;">
                <span>AI-anrop</span>
                <strong>${usageCount}/${usageLimit}</strong>
            </div>
            <div class="usage-mini-bar"><div class="usage-mini-fill" style="width:${pct}%;background:${color};"></div></div>
            ${remaining === 0 ? '<a href="#" onclick="event.preventDefault();showView(\'settings\');renderSubscriptionStatus();" style="color:#ef4444;font-size:0.75rem;">Uppgradera</a>' : ''}
        `;
    }
}

async function renderStripePricingTable() {
    const container = document.getElementById('stripe-pricing-table-container');
    if (!container) return;

    // Fetch the public stripe config from the server.
    try {
        const res = await fetch('/api/billing/config');
        if (!res.ok) return;
        const cfg = await res.json();
        if (!cfg.publishable_key || !cfg.pricing_table_id) {
            container.innerHTML = '<p class="muted">Prenumerationsuppgradering är inte konfigurerad.</p>';
            return;
        }
        const user = state.user;
        const table = document.createElement('stripe-pricing-table');
        table.setAttribute('pricing-table-id', cfg.pricing_table_id);
        table.setAttribute('publishable-key', cfg.publishable_key);
        if (user) {
            table.setAttribute('client-reference-id', user.id);
            table.setAttribute('customer-email', user.email);
        }
        container.innerHTML = '';
        container.appendChild(table);
    } catch (err) {
        console.error('Failed to load pricing table config', err);
    }
}

async function openBillingPortal() {
    try {
        const res = await fetch('/api/billing/portal', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
        });
        if (!res.ok) {
            const msg = await res.text();
            alert(msg || 'Kunde inte öppna kundportalen');
            return;
        }
        const data = await res.json();
        if (data.url) {
            window.location.href = data.url;
        }
    } catch (err) {
        console.error('portal error', err);
        alert('Kunde inte öppna kundportalen');
    }
}

async function deleteListing(id) {
    if (!id) return;
    const ok = window.confirm('Ta bort detta objekt?');
    if (!ok) return;
    try {
        const res = await fetch(`/api/listings/${id}/`, { method: 'DELETE' });
        if (!res.ok) {
            const txt = await res.text();
            throw new Error(txt || 'Misslyckades med att ta bort objekt');
        }
        if (state.selectedId === id) {
            state.selectedId = null;
            state.current = null;
            renderDetail();
        }
        await fetchListings();
    } catch (err) {
        alert(err.message);
    }
}

async function openListingModal(id) {
    if (!id) return;
    let detail = null;
    if (state.current && state.current.id === id) {
        detail = state.current;
    } else {
        try {
            const res = await fetch(`/api/listings/${id}/`);
            if (!res.ok) throw new Error('Kunde inte hämta objekt');
            detail = await res.json();
        } catch (err) {
            alert(err.message);
            return;
        }
    }
    const overlay = document.getElementById('modal-overlay');
    const title = document.getElementById('modal-title');
    const textarea = document.getElementById('modal-textarea');
    title.textContent = detail.address || 'Objekt';
    if (textarea) textarea.value = getFullCopy(detail) || 'Ingen text ännu.';
    overlay.classList.remove('hidden');
}

function closeModal() {
    document.getElementById('modal-overlay')?.classList.add('hidden');
}

