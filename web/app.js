const state = {
    listings: [],
    current: null,
    selectedId: null,
    versions: [],
    lastText: '',
    uploads: [],
    listingFilter: '',
    visionAnalysis: null,
    visionDesign: null,
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

async function fetchListings() {
    try {
        const res = await fetch('/api/listings/');
        if (!res.ok) throw new Error('Kunde inte hämta listor');
        state.listings = await res.json();
        updateVolumeStats();
        updateTimeSavings();
        updateImageStats();
        renderObjectList();
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

function buildPayloadFromForm() {
    const highlights = listFromLines(value('highlights'));
    return {
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
        highlights,
        target_audience: 'Bred målgrupp',
        fee: 0,
    };
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
        await fetchListings();
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

    if (!detail) {
        header.textContent = 'Ingen annons än';
        editor.value = '';
        editor.readOnly = true;
        copyBtn.disabled = true;
        downloadBtn.disabled = true;
        regenerateBtn.disabled = true;
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
    if (!promptEl) return;
    const prompt = promptEl.value.trim();
    if (!prompt) {
        setVisionStatus('design', 'Beskriv ditt önskemål först.', true);
        return;
    }
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

function setVisionStatus(type, message, isError) {
    const id = type === 'design' ? 'vision-design-status' : 'vision-analyze-status';
    const el = document.getElementById(id);
    if (!el) return;
    el.textContent = message || '';
    el.classList.toggle('error', Boolean(isError));
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
        const res = await fetch(`/api/listings/${state.selectedId}/sections/main/rewrite`, {
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

function bindEvents() {
    const form = document.getElementById('listing-form');
    form.addEventListener('submit', handleCreate);
    document.getElementById('property-type').addEventListener('change', toggleFloorField);

    document.querySelectorAll('.selection-action').forEach(btn => {
        btn.addEventListener('click', () => applySelectionRewrite(btn.dataset.mode));
    });
    document.getElementById('regenerate-btn').addEventListener('click', regenerateWithTone);
    document.getElementById('copy-text-btn').addEventListener('click', copyFullText);
    document.getElementById('download-txt-btn').addEventListener('click', downloadText);
    document.getElementById('upload-btn').addEventListener('click', () => document.getElementById('file-input').click());
    document.getElementById('file-input').addEventListener('change', handleFiles);
    const visionAnalyzeForm = document.getElementById('vision-analyze-form');
    if (visionAnalyzeForm) {
        visionAnalyzeForm.addEventListener('submit', handleVisionAnalyze);
    }
    const visionDesignForm = document.getElementById('vision-design-form');
    if (visionDesignForm) {
        visionDesignForm.addEventListener('submit', handleVisionDesign);
    }

    const dropzone = document.getElementById('dropzone');
    dropzone.addEventListener('click', () => document.getElementById('file-input').click());
    dropzone.addEventListener('dragover', e => { e.preventDefault(); dropzone.classList.add('dragging'); });
    dropzone.addEventListener('dragleave', () => dropzone.classList.remove('dragging'));
    dropzone.addEventListener('drop', e => {
        e.preventDefault();
        dropzone.classList.remove('dragging');
        handleFiles({ target: { files: e.dataTransfer.files } });
    });

    document.getElementById('clear-versions').addEventListener('click', () => {
        state.versions = [];
        renderVersions();
    });

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
        btn.addEventListener('click', () => showView(btn.dataset.viewTrigger));
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
        }
    });
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

        const title = document.createElement('div');
        title.className = 'object-card__title';
        title.textContent = listing.address || 'Namnlöst objekt';

        const meta = document.createElement('div');
        meta.className = 'object-card__meta';
        meta.textContent = buildListingMeta(listing);

        const status = document.createElement('div');
        status.className = 'object-card__status';
        status.textContent = buildListingStatus(listing);

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

    if (Array.isArray(listing.highlights)) {
        setValue('highlights', listing.highlights.join(', '));
    } else if (listing.highlights) {
        setValue('highlights', listing.highlights);
    } else {
        setValue('highlights', '');
    }
    toggleFloorField();
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

function handleFiles(event) {
    const files = Array.from(event.target.files || []);
    if (!files.length) return;
    files.forEach(file => {
        state.uploads.push({ name: file.name, status: 'Analyserar...', size: file.size });
    });
    renderUploads();
    simulateImageAnalysis();
}

function renderUploads() {
    const list = document.getElementById('upload-list');
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

function simulateImageAnalysis() {
    const status = document.getElementById('ai-status');
    setAIStatus('Analyserar bilder och filer...', true);
    setTimeout(() => {
        state.uploads = state.uploads.map(file => ({ ...file, status: 'Analyserad' }));
        renderUploads();
        updateImageStats();
        setAIStatus('Bildanalys klar. Förbättrade ton och detaljer i texten.', false, true);
    }, 1200);
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
    populateFormFromListing(detail);
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
    const processed = state.uploads.filter(file => file.status === 'Analyserad').length;
    const el = document.getElementById('stat-images');
    const avgEl = document.getElementById('stat-images-avg');
    const listingCount = state.listings.length;
    const average = listingCount ? processed / listingCount : 0;

    if (el) el.textContent = processed;
    if (avgEl) avgEl.textContent = average.toFixed(1);
}

bindEvents();
renderVisionLab();
showView('objects');
fetchListings();

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
