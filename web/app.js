const state = {
    user: null,
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
    const sidebarLabel = document.getElementById('user-email-sidebar');
    if (sidebarLabel) {
        sidebarLabel.textContent = user?.email || 'Inloggad';
    }
    enterAppShell();
}

function resetAppData() {
    state.listings = [];
    state.current = null;
    state.selectedId = null;
    renderObjectList();
    renderDetail();
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
    return res;
};

async function fetchListings() {
    try {
        const res = await fetch('/api/listings/');
        if (!res.ok) throw new Error('Kunde inte hämta listor');
        state.listings = await res.json();
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
        state.styleProfiles = await res.json();
        renderStyleProfileOptions();
        renderStyleProfileList();
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
        highlights,
        target_audience: 'Bred målgrupp',
        fee: 0,
        images,
    };
    state.selectedProfileId = payload.style_profile_id || '';
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
    setAIStatus('Genererar annons med vald ton...', true);
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
        await fetchListings();
        state.uploads = [];
        renderUploads();
        document.getElementById('form-message').textContent = 'Annons genererad.';
        setAIStatus('Klar. Du kan markera texten och omskriva.', false, true);
    } catch (err) {
        alert(err.message);
        setAIStatus('', false, true);
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

async function handleVisionAnalyze(event) {
    event.preventDefault();
    const input = document.getElementById('vision-image-url');
    const fileInput = document.getElementById('vision-image-file');
    if (!input) return;
    const imageURL = input.value.trim();
    const file = fileInput?.files?.[0];
    if (!imageURL && !file) {
        setVisionStatus('analyze', 'Ange en länk eller välj en bildfil.', true);
        return;
    }
    if (!imageURL) {
        setVisionStatus('analyze', 'Analyserar bild...', false);
    } else {
        setVisionStatus('analyze', 'Analyserar bild...', false);
    }
    try {
        let res;
        if (file) {
            const formData = new FormData();
            formData.append('image_file', file);
            if (imageURL) {
                formData.append('image_url', imageURL);
            }
            res = await fetch('/api/vision/analyze', {
                method: 'POST',
                body: formData,
            });
        } else {
            res = await fetch('/api/vision/analyze', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ image_url: imageURL }),
            });
        }
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || 'Misslyckades med bildanalys');
        }
        state.visionAnalysis = await res.json();
        setVisionStatus('analyze', 'Analysen är klar.', false);
        if (fileInput) {
            fileInput.value = '';
        }
        renderVisionLab();
    } catch (err) {
        setVisionStatus('analyze', err.message, true);
    }
}

async function handleVisionDesign(event) {
    event.preventDefault();
    const promptEl = document.getElementById('vision-design-prompt');
    const styleEl = document.getElementById('vision-style');
    if (!promptEl || !styleEl) return;
    const extra = promptEl.value.trim();
    const style = styleEl.value;
    const analysis = state.visionRemix.analysis;
    if (!analysis && !extra) {
        setVisionStatus('design', 'Ladda upp en bild eller skriv en instruktion.', true);
        return;
    }
    const prompt = buildDesignPrompt(style, extra, analysis);
    setVisionStatus('design', 'Skapar designförslag...', false);
    try {
        const res = await fetch('/api/vision/design', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ prompt }),
        });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || 'Misslyckades med designförslag');
        }
        state.visionDesign = await res.json();
        setVisionStatus('design', 'Designförslaget är klart.', false);
        renderVisionLab();
    } catch (err) {
        setVisionStatus('design', err.message, true);
    }
}

async function handleVisionRender() {
    const promptEl = document.getElementById('vision-design-prompt');
    const styleEl = document.getElementById('vision-style');
    if (!styleEl) return;
    const extra = promptEl?.value.trim() || '';
    const style = styleEl.value;
    const analysis = state.visionRemix.analysis;
    if (!analysis && !extra && !state.visionDesign) {
        setVisionStatus('render', 'Skapa först en designidé eller skriv instruktioner.', true);
        return;
    }
    const prompt = buildImagePrompt(style, extra, analysis);
    setVisionStatus('render', 'Genererar visualisering...', false);
    state.visionRenderImage = '';
    state.visionRenderAsset = null;
    renderVisionLab();
    const baseImageData = (state.visionRemix.previewURL || '').startsWith('data:')
        ? state.visionRemix.previewURL
        : '';
    const baseImageURL = !baseImageData
        ? state.visionRemix.storageURL || state.current?.image_url || ''
        : '';
    try {
        const res = await fetch('/api/vision/render', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                prompt,
                base_image_data: baseImageData,
                base_image_url: baseImageURL,
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
        setVisionStatus('render', 'Visualisering klar.', false);
        renderVisionLab();
    } catch (err) {
        state.visionRenderImage = '';
        state.visionRenderAsset = null;
        setVisionStatus('render', err.message, true);
        renderVisionLab();
    }
}

async function attachRenderToListing() {
    const select = document.getElementById('vision-render-listing');
    const statusEl = document.getElementById('vision-render-link-status');
    const noteEl = document.getElementById('vision-render-note');
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
        const label = noteEl?.value.trim() ? `AI: ${noteEl.value.trim()}` : 'AI-rendering';
        await attachImageToListing(listingId, {
            url: asset.url,
            key: asset.key,
            source: 'ai',
            kind: 'render',
            label,
        });
        statusEl.textContent = 'Bilden kopplades till objektet.';
        if (noteEl) noteEl.value = '';
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

function setVisionStatus(type, message, isError) {
    const mapping = {
        design: 'vision-design-status',
        analyze: 'vision-analyze-status',
        remix: 'vision-remix-status',
        render: 'vision-render-status',
    };
    const id = mapping[type] || mapping.analyze;
    const el = document.getElementById(id);
    if (!el) return;
    el.textContent = message || '';
    el.classList.toggle('error', Boolean(isError));
}

function buildDesignPrompt(style, extra, analysis) {
    const parts = [];
    if (analysis) {
        const descriptors = [];
        if (analysis.summary) {
            descriptors.push(analysis.summary);
        }
        const tagList = []
            .concat(analysis.notable_details || [])
            .concat(analysis.tags || []);
        if (tagList.length) {
            descriptors.push(`Detaljer att tänka på: ${tagList.join(', ')}`);
        }
        if ((analysis.color_palette || []).length) {
            descriptors.push(`Nuvarande färger: ${(analysis.color_palette || []).join(', ')}`);
        }
        const base = [
            `Utgångsrum: ${analysis.room_type || 'okänt'}`,
            analysis.style ? `Upplevd stil: ${analysis.style}` : null,
        ].filter(Boolean).join('. ');
        parts.push(`${base}. ${descriptors.join(' ')}`);
    }
    if (style) {
        parts.push(`Designen ska kännas "${style}".`);
    }
    if (extra) {
        parts.push(`Extra instruktioner från mäklaren: ${extra}`);
    }
    parts.push('Lista vilka möbler, textilier, belysning och dekor som ska läggas till så att ett tomt rum blir komplett.');
    return parts.join('\n');
}

function buildImagePrompt(style, extra, analysis) {
    const parts = [];
    if (analysis) {
        parts.push(`Utgå från ett ${analysis.room_type || 'inomhusrum'} som beskrivs så här: ${analysis.summary || ''}`);
        if ((analysis.color_palette || []).length) {
            parts.push(`Nuvarande färger: ${(analysis.color_palette || []).join(', ')}`);
        }
        if ((analysis.notable_details || []).length) {
            parts.push(`Detaljer som ska bevaras: ${(analysis.notable_details || []).join(', ')}`);
        }
        parts.push('Rendera exakt samma kameravinkel/perspektiv som utgångsbilden. Ändra aldrig väggarnas form, fönstrens placering, dörröppningar eller rummets proportioner.');
        parts.push('Den enda förändringen ska vara möbler, textilier, belysning och dekor. Arkitekturen, fönster, dörrar och ljusinsläpp måste ligga kvar där de är i originalbilden.');
        parts.push('Om instruktionen bryts (t.ex. fönster flyttas eller kameran vrids) ska du avvisa ändringen och i stället beskriva hur möblerna placeras med den exakta layouten.');
    }
    if (style) {
        parts.push(`Övergripande känsla: ${style}.`);
    }
    if (extra) {
        parts.push(`Extra instruktioner: ${extra}`);
    }
    const concept = state.visionDesign;
    if (concept) {
        if (concept.layout) parts.push(`Layoutidé: ${concept.layout}`);
        if ((concept.items || []).length) parts.push(`Ny inredning: ${(concept.items || []).join(', ')}`);
        if ((concept.palette || []).length) parts.push(`Önskade färger: ${(concept.palette || []).join(', ')}`);
        if (concept.lighting) parts.push(`Ljus: ${concept.lighting}`);
    }
    parts.push('Rendera en fotorealistisk interiörbild, 4K-upplösning, naturligt ljus, visa hela rummet.');
    return parts.join('\n');
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

function renderVisionLab() {
    const analyzeEl = document.getElementById('vision-analyze-result');
    if (analyzeEl) {
        const data = state.visionAnalysis;
        if (!data) {
            analyzeEl.innerHTML = '<p class="muted">Ingen analys ännu.</p>';
        } else {
            const tags = []
                .concat(data.notable_details || [])
                .concat((data.color_palette || []).map(color => `Färg: ${color}`))
                .concat(data.tags || []);
            analyzeEl.innerHTML = `
                <h4>${data.room_type || 'Oidentifierat rum'}</h4>
                <p>${data.summary || 'Inga detaljer angavs.'}</p>
                <p><strong>Stil:</strong> ${data.style || 'Okänd'}</p>
                ${tags.length ? `<div class="vision-badges">${tags.map(tag => `<span class="vision-badge">${tag}</span>`).join('')}</div>` : '<p class="muted">Inga etiketter.</p>'}
            `;
        }
    }

    const designEl = document.getElementById('vision-design-output');
    if (designEl) {
        const concept = state.visionDesign;
        if (!concept) {
            designEl.innerHTML = '<p class="muted">Inget designförslag ännu.</p>';
        } else {
            const buildList = (items, label) => {
                if (!items || !items.length) return '';
                return `<p class="vision-result__label">${label}</p><ul>${items.map(item => `<li>${item}</li>`).join('')}</ul>`;
            };
            designEl.innerHTML = `
                <h4>${concept.mood || 'Designförslag'}</h4>
                <p>${concept.summary || ''}</p>
                ${concept.layout ? `<p><strong>Layout:</strong> ${concept.layout}</p>` : ''}
                ${concept.lighting ? `<p><strong>Belysning:</strong> ${concept.lighting}</p>` : ''}
                ${buildList(concept.items, 'Möbler & element')}
                ${buildList(concept.palette, 'Färgpalett')}
                ${buildList(concept.notes, 'Noteringar')}
            `;
        }
    }

    const remixPreview = document.getElementById('vision-remix-preview');
    if (remixPreview) {
        if (state.visionRemix.previewURL) {
            remixPreview.innerHTML = `<img src="${state.visionRemix.previewURL}" alt="Förhandsvisning av rum">`;
        } else {
            remixPreview.innerHTML = '<p>Ladda upp eller släpp en bild här</p>';
        }
    }

    const remixAnalysis = document.getElementById('vision-remix-analysis');
    if (remixAnalysis) {
        const insight = state.visionRemix.analysis;
        if (!insight) {
            remixAnalysis.innerHTML = '<p class="muted">Ingen bild ännu.</p>';
        } else {
            const badges = []
                .concat(insight.notable_details || [])
                .concat(insight.tags || [])
                .slice(0, 6);
            remixAnalysis.innerHTML = `
                <h4>${insight.room_type || 'Okänt rum'}</h4>
                <p>${insight.summary || 'Ingen sammanfattning.'}</p>
                ${insight.style ? `<p><strong>Stil:</strong> ${insight.style}</p>` : ''}
                ${(insight.color_palette || []).length ? `<p><strong>Färger:</strong> ${(insight.color_palette || []).join(', ')}</p>` : ''}
                ${badges.length ? `<div class="vision-badges">${badges.map(tag => `<span class="vision-badge">${tag}</span>`).join('')}</div>` : ''}
            `;
        }
    }

    const renderOutput = document.getElementById('vision-render-output');
    if (renderOutput) {
        if (state.visionRenderImage) {
            renderOutput.innerHTML = `<img src="${state.visionRenderImage}" alt="AI-genererad visualisering">`;
        } else {
            renderOutput.innerHTML = '<p class="muted">Ingen bild genererad ännu.</p>';
        }
    }
    const renderSelect = document.getElementById('vision-render-listing');
    if (renderSelect) {
        const prev = renderSelect.value;
        renderSelect.innerHTML = '<option value="">Välj objekt</option>' + state.listings.map(item => `<option value="${item.id}">${item.address || 'Namnlöst objekt'}</option>`).join('');
        if (prev && state.listings.some(item => item.id === prev)) {
            renderSelect.value = prev;
        }
    }
    const attachBtn = document.getElementById('vision-render-attach');
    if (attachBtn) {
        attachBtn.disabled = !state.visionRenderImage;
    }
    const linkStatus = document.getElementById('vision-render-link-status');
    if (linkStatus && !state.visionRenderImage) {
        linkStatus.textContent = '';
        linkStatus.classList.remove('error');
    }
}

function handleRemixImageChange(event) {
    const file = event.target.files?.[0];
    if (file) {
        processRemixFile(file);
    }
}

function processRemixFile(file) {
    if (!file.type.startsWith('image/')) {
        setVisionStatus('remix', 'Välj en bildfil (jpg/png).', true);
        return;
    }
    state.visionRemix.file = file;
    state.visionRemix.analysis = null;
    state.visionRemix.storageURL = '';
    state.visionRemix.storageKey = '';
    const reader = new FileReader();
    reader.onload = () => {
        state.visionRemix.previewURL = reader.result;
        renderVisionLab();
    };
    reader.readAsDataURL(file);
    analyzeRemixImage(file);
}

async function analyzeRemixImage(file) {
    if (!file) return;
    const formData = new FormData();
    formData.append('image_file', file);
    setVisionStatus('remix', 'Analyserar rummet...', false);
    try {
        const res = await fetch('/api/vision/analyze', {
            method: 'POST',
            body: formData,
        });
        if (!res.ok) {
            const txt = await res.text();
            throw new Error(txt || 'Kunde inte analysera bilden');
        }
        state.visionRemix.analysis = await res.json();
        setVisionStatus('remix', 'Rummet är förstått.', false);
        renderVisionLab();
    } catch (err) {
        state.visionRemix.analysis = null;
        setVisionStatus('remix', err.message, true);
    }
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
            const copy = mode === 'register'
                ? 'Skapa konto och gå direkt till baksidan.'
                : 'Logga in för att öppna baksidan.';
            showAuthOverlay(copy);
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
    const visionAnalyzeForm = document.getElementById('vision-analyze-form');
    if (visionAnalyzeForm) {
        visionAnalyzeForm.addEventListener('submit', handleVisionAnalyze);
    }
    const visionDesignForm = document.getElementById('vision-design-form');
    if (visionDesignForm) {
        visionDesignForm.addEventListener('submit', handleVisionDesign);
    }
    const visionRenderBtn = document.getElementById('vision-render-btn');
    if (visionRenderBtn) {
        visionRenderBtn.addEventListener('click', handleVisionRender);
    }
    const visionRenderAttachBtn = document.getElementById('vision-render-attach');
    if (visionRenderAttachBtn) {
        visionRenderAttachBtn.addEventListener('click', attachRenderToListing);
    }
    const remixImageInput = document.getElementById('vision-remix-image');
    if (remixImageInput) {
        remixImageInput.addEventListener('change', handleRemixImageChange);
    }
    const remixPreview = document.getElementById('vision-remix-preview');
    if (remixPreview) {
        remixPreview.addEventListener('dragover', e => {
            e.preventDefault();
            remixPreview.classList.add('dragging');
        });
        remixPreview.addEventListener('dragleave', () => remixPreview.classList.remove('dragging'));
        remixPreview.addEventListener('drop', e => {
            e.preventDefault();
            remixPreview.classList.remove('dragging');
            const file = e.dataTransfer?.files?.[0];
            if (file) {
                processRemixFile(file);
            }
        });
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
            if (view === 'generator') {
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
    toggleFloorField();
    const styleSelect = document.getElementById('style-profile');
    if (styleSelect) {
        styleSelect.value = state.selectedProfileId || '';
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

async function extractAnnualReport(file, timeoutMs = 90000) {
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
    `;
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

    const total = state.listings.length;
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
        images: {
            title: 'Bildhantering',
            subtitle: 'Hantera och ladda upp bildmaterial.',
        },
        templates: {
            title: 'Mallar',
            subtitle: 'Återanvänd strukturer och tonlägen.',
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
checkSession();

async function initApp() {
    if (!state.user) return;
    await fetchStyleProfiles();
    await fetchListings();
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
