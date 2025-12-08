const QUICK_PROMPTS = {
    intro: [
        { label: 'Kortare', instruction: 'Förkorta inledningen men behåll det emotionella tonläget.' },
        { label: 'Premium', instruction: 'Ge texten en mer exklusiv ton och lyft fram unika kvaliteter.' },
        { label: 'Faktabaserad', instruction: 'Gör texten mer faktabaserad och rak utan att tappa flytet.' },
        { label: 'Lyft puff', instruction: 'Skapa en kort pufftext som sammanfattar objektet på 2 meningar.' }
    ],
    hall: [
        { label: 'Förvaring', instruction: 'Beskriv hallens förvaring mer ingående och praktiskt.' },
        { label: 'Välkomnande', instruction: 'Fokusera på känslan av att komma hem och ljuset i hallen.' },
        { label: 'Kort', instruction: 'Korta ner hallbeskrivningen till 1-2 meningar med skärpa.' }
    ],
    kitchen: [
        { label: 'Matlagning', instruction: 'Betona kökets funktioner för den som gillar att laga mat.' },
        { label: 'Socialt', instruction: 'Beskriv hur köket öppnar upp för sociala middagar.' },
        { label: 'Lyx', instruction: 'Gör tonen lyxig och materialinriktad med fokus på detaljer.' }
    ],
    living: [
        { label: 'Familj', instruction: 'Gör texten mer familjär och betona plats för umgänge.' },
        { label: 'Design', instruction: 'Lyft fram design, ljus och material i vardagsrummet.' },
        { label: 'Kort', instruction: 'Kondensera stycket till det viktigaste i 2 meningar.' }
    ],
    area: [
        { label: 'Barnfamilj', instruction: 'Betona skolor, parker och trygga gårdar i området.' },
        { label: 'Citypuls', instruction: 'Fokusera på kommunikationer och restauranger i närområdet.' },
        { label: 'Natur', instruction: 'Förtydliga närhet till natur, bad eller motionsspår.' }
    ]
};

const DEFAULT_PROMPT = 'Polera texten och gör den mer säljande utan att ändra fakta.';

const state = {
    listings: [],
    current: null,
    selectedId: null
};

const LENGTH_LABELS = {
    1: 'kort',
    2: 'normal',
    3: 'lång'
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

function buildInstructionSummary(form) {
    const parts = [
        `Ton: ${form.tone || 'neutral'}.`,
        `Önskad längd: ${LENGTH_LABELS[form.length] || 'normal'}.`,
    ];
    if (form.targetAudience) parts.push(`Målgrupp: ${form.targetAudience}.`);
    if (form.propertyType) parts.push(`Bostadstyp: ${form.propertyType}.`);
    if (form.plan) parts.push(`Planlösning: ${form.plan}.`);
    if (form.instructions) parts.push(`Extra: ${form.instructions}`);
    return parts.join(' ');
}

function buildSectionsFromForm(form) {
    const sections = [];
    const locationBits = [form.neighborhood, form.city, form.municipality].filter(Boolean).join(', ');
    const factLine = [
        form.rooms ? `${form.rooms} rum` : '',
        form.livingArea ? `${form.livingArea} kvm` : '',
        form.floor,
    ]
        .filter(Boolean)
        .join(' · ');

    const introTone = {
        Neutral: 'faktabaserad',
        'Säljande': 'säljande',
        Lyxig: 'sofistikerad',
        Familjär: 'varm och välkomnande',
        Faktabaserad: 'rak och tydlig',
    }[form.tone] || 'balanserad';

    const intro = [
        `Välkommen till ${form.address}${locationBits ? ` i ${locationBits}` : ''}.`,
        factLine ? `Här erbjuds ${factLine} i en ${introTone} tappning.` : `Texten hålls ${introTone}.`,
        form.plan ? `Planlösningen är ${form.plan.toLowerCase()}.` : '',
        form.highlights.length ? `Highlights: ${form.highlights.join(', ')}.` : '',
    ]
        .filter(Boolean)
        .join(' ');

    const plan = [
        form.plan ? `Planlösning: ${form.plan}.` : '',
        form.storage ? `Förvaring: ${form.storage}.` : '',
        form.parking ? `Parkering: ${form.parking}.` : '',
        form.fee ? `Avgift: ${form.fee.toLocaleString('sv-SE')} kr/mån.` : '',
        form.operatingCost ? `Drift: ${form.operatingCost}.` : '',
        form.price ? `Pris: ${form.price}.` : '',
    ]
        .filter(Boolean)
        .join(' ');

    const interiorParts = [
        form.kitchen ? `Kök: ${form.kitchen}` : '',
        form.living ? `Vardagsrum: ${form.living}` : '',
        form.bedroom ? `Sovrum: ${form.bedroom}` : '',
        form.bathroom ? `Badrum: ${form.bathroom}` : '',
        form.outdoor ? `Ute: ${form.outdoor}` : '',
    ].filter(Boolean);

    const association = [
        form.associationName ? `Förening: ${form.associationName}.` : '',
        form.associationEconomy ? `Ekonomi: ${form.associationEconomy}` : '',
        form.renovationsDone ? `Genomfört: ${form.renovationsDone}.` : '',
        form.renovationsPlanned ? `Planerat: ${form.renovationsPlanned}.` : '',
        form.commonAreas ? `Gemensamt: ${form.commonAreas}.` : '',
    ]
        .filter(Boolean)
        .join(' ');

    const area = [
        form.communications ? `Kommunikationer: ${form.communications}.` : '',
        form.service ? `Service: ${form.service}.` : '',
        form.schools ? `Skolor/förskolor: ${form.schools}.` : '',
        form.nature ? `Natur: ${form.nature}.` : '',
    ]
        .filter(Boolean)
        .join(' ');

    const advantagesContent = form.advantages.length
        ? form.advantages.map(item => `• ${item}`).join('\n')
        : '';

    sections.push({ slug: 'intro', title: 'Inledning', content: intro });
    if (interiorParts.length) {
        sections.push({ slug: 'interior', title: 'Interiör', content: interiorParts.join('\n\n') });
    }
    if (plan) {
        sections.push({ slug: 'plan', title: 'Planlösning & ekonomi', content: plan });
    }
    if (association) {
        sections.push({ slug: 'association', title: 'Förening', content: association });
    }
    if (area) {
        sections.push({ slug: 'area', title: 'Område', content: area });
    }
    if (advantagesContent) {
        sections.push({ slug: 'advantages', title: 'Fördelar', content: advantagesContent });
    }

    return sections;
}

function resetDetailPanel(message = 'Välj ett objekt för att visa detaljer') {
    state.current = null;
    const addressEl = document.getElementById('detail-address');
    const metaEl = document.getElementById('detail-meta');
    const statusEl = document.getElementById('detail-status');
    const bodyEl = document.getElementById('detail-body');
    const copyBtn = document.getElementById('copy-text-btn');
    const copyHtmlBtn = document.getElementById('copy-html-btn');
    const downloadBtn = document.getElementById('download-txt-btn');
    const deleteBtn = document.getElementById('delete-listing-btn');
    const addBtn = document.getElementById('add-section-btn');

    addressEl.textContent = 'Välj ett objekt för att visa detaljer';
    metaEl.textContent = '';
    statusEl.innerHTML = '';
    bodyEl.innerHTML = `<p class="muted">${message}</p>`;

    copyBtn.disabled = true;
    copyHtmlBtn.disabled = true;
    downloadBtn.disabled = true;
    deleteBtn.disabled = true;
    addBtn.disabled = true;
}

async function fetchListings() {
    const listEl = document.getElementById('listing-list');
    if (listEl) {
        listEl.innerHTML = '<li class="list__item">Hämtar...</li>';
    }
    const gridEl = document.getElementById('listing-grid');
    if (gridEl) {
        gridEl.classList.remove('empty');
        gridEl.innerHTML = '';
    }

    try {
        const res = await fetch('/api/listings/');
        if (!res.ok) throw new Error('Kunde inte hämta listor');
        state.listings = await res.json();
        renderListingList();
        renderListingGrid();

        if (!state.listings.length) {
            resetDetailPanel('Inga objekt ännu. Skapa ett första!');
            return;
        }

        if (!state.selectedId || !state.listings.find(l => l.id === state.selectedId)) {
            await selectListing(state.listings[0].id);
        } else if (state.selectedId) {
            await selectListing(state.selectedId);
        }
    } catch (err) {
        if (listEl) listEl.innerHTML = `<li class="list__item">${err.message}</li>`;
        if (gridEl) {
            gridEl.classList.add('empty');
            gridEl.innerHTML = '';
        }
    }
}

function renderListingGrid() {
    const gridEl = document.getElementById('listing-grid');
    if (!gridEl) return;

    gridEl.innerHTML = '';
    if (!state.listings.length) {
        gridEl.classList.add('empty');
        return;
    }
    gridEl.classList.remove('empty');

    state.listings.forEach(item => {
        const card = document.createElement('article');
        card.className = 'listing-card';
        if (item.id === state.selectedId) {
            card.classList.add('active');
        }
        card.dataset.id = item.id;

        const facts = [];
        if (item.rooms) {
            const roomsValue = Number(item.rooms);
            const formattedRooms = Number.isInteger(roomsValue) ? roomsValue.toString() : roomsValue.toFixed(1).replace('.0', '');
            facts.push(`${formattedRooms} rum`);
        }
        if (item.living_area) {
            const livingArea = Number(item.living_area);
            facts.push(`${livingArea.toLocaleString('sv-SE', { maximumFractionDigits: 1 })} kvm`);
        }
        if (item.fee) {
            facts.push(`${Number(item.fee).toLocaleString('sv-SE')} kr/mån`);
        }

        const highlights = item.highlights?.slice?.(0, 3) || [];
        const price = item.price || '';
        const location = [item.neighborhood, item.city, item.municipality].filter(Boolean).join(', ');
        const badge = item.tone || 'Neutral';
        const imageLabel = (item.property_type || 'Objekt').toUpperCase();

        card.innerHTML = `
            <div class="listing-card__image">
                <span>${imageLabel}</span>
            </div>
            <div class="listing-card__body">
                <div class="listing-card__meta">
                    <span class="badge">${badge}</span>
                    ${location ? `<span>${location}</span>` : ''}
                </div>
                <h3>${item.address || 'Okänd adress'}</h3>
                ${facts.length ? `<div class="card-facts">${facts.join(' · ')}</div>` : ''}
                ${price ? `<div class="price-tag">${price}</div>` : ''}
                ${highlights.length ? `<div class="card-tags">${highlights.map(tag => `<span class="card-tag">${tag}</span>`).join('')}</div>` : ''}
            </div>
        `;

        card.addEventListener('click', () => selectListing(item.id));
        gridEl.appendChild(card);
    });
}

function renderListingList() {
    const listEl = document.getElementById('listing-list');
    if (!state.listings.length) {
        listEl.innerHTML = '<li class="list__item">Inga objekt ännu. Skapa ett första!</li>';
        return;
    }

    listEl.innerHTML = '';
    state.listings.forEach(item => {
        const li = document.createElement('li');
        li.className = 'list__item';
        if (item.id === state.selectedId) {
            li.classList.add('active');
        }
        li.dataset.id = item.id;

        const facts = [];
        if (item.living_area) {
            const livingArea = Number(item.living_area);
            facts.push(`${livingArea.toLocaleString('sv-SE', { maximumFractionDigits: 1 })} kvm`);
        }
        if (item.rooms) {
            const roomsValue = Number(item.rooms);
            const formattedRooms = Number.isInteger(roomsValue) ? roomsValue.toString() : roomsValue.toFixed(1);
            facts.push(`${formattedRooms.replace('.0', '')} rum`);
        }
        if (item.fee) {
            facts.push(`${Number(item.fee).toLocaleString('sv-SE')} kr/mån`);
        }
        const highlights = item.highlights?.length ? `<p>${item.highlights.join(', ')}</p>` : '';
        li.innerHTML = `
            <h3>${item.address}</h3>
            <div class="list__meta">
                <span class="badge">${item.tone}</span>
                <span>${item.target_audience}</span>
                <span>${new Date(item.created_at).toLocaleString('sv-SE')}</span>
            </div>
            ${facts.length ? `<div class="list__facts">${facts.join(' · ')}</div>` : ''}
            ${highlights}
        `;
        listEl.appendChild(li);
    });
}

async function selectListing(id) {
    if (!id) return;
    state.selectedId = id;
    const listItems = document.querySelectorAll('#listing-list .list__item');
    listItems.forEach(item => {
        if (item.dataset.id === id) {
            item.classList.add('active');
        } else {
            item.classList.remove('active');
        }
    });

    try {
        const res = await fetch(`/api/listings/${id}/`);
        if (!res.ok) throw new Error('Kunde inte hämta objekt');
        state.current = await res.json();
        renderDetail(state.current);
    } catch (err) {
        const body = document.getElementById('detail-body');
        body.innerHTML = `<p class="muted">${err.message}</p>`;
    }
}

function renderDetail(detail) {
    const addressEl = document.getElementById('detail-address');
    const metaEl = document.getElementById('detail-meta');
    const statusEl = document.getElementById('detail-status');
    const bodyEl = document.getElementById('detail-body');
    const copyBtn = document.getElementById('copy-text-btn');
    const copyHtmlBtn = document.getElementById('copy-html-btn');
    const downloadTxtBtn = document.getElementById('download-txt-btn');
    const deleteBtn = document.getElementById('delete-listing-btn');
    const addBtn = document.getElementById('add-section-btn');

    addressEl.textContent = detail.address;

    const facts = [];
    if (detail.living_area) facts.push(`${detail.living_area} kvm`);
    if (detail.rooms) facts.push(`${detail.rooms} rum`);
    if (detail.fee) facts.push(`${Number(detail.fee).toLocaleString('sv-SE')} kr/mån`);
    metaEl.textContent = [detail.tone, detail.target_audience, ...facts].filter(Boolean).join(' · ');

    const hasCopy = Boolean(getFullCopy(detail));
    copyBtn.disabled = !hasCopy;
    copyHtmlBtn.disabled = !hasCopy;
    downloadTxtBtn.disabled = !hasCopy;
    deleteBtn.disabled = false;
    addBtn.disabled = false;
    renderStatus(detail, statusEl);

    if (!detail.sections?.length) {
        bodyEl.innerHTML = '<p class="muted">Inga sektioner ännu. Skicka en omskrivning eller generera ett nytt utkast.</p>';
        return;
    }

    bodyEl.innerHTML = '';
        detail.sections.forEach(section => {
            const wrapper = document.createElement('article');
            wrapper.className = 'section-editor';
            wrapper.dataset.slug = section.slug;
            wrapper.innerHTML = `
            <header>
                <div>
                    <p class="eyebrow">${section.slug}</p>
                    <h3>${section.title}</h3>
                </div>
                <button type="button" class="delete-section" data-slug="${section.slug}">Ta bort</button>
            </header>
            <textarea>${section.content || 'Ingen text genererad än.'}</textarea>
            <div class="selection-tools">
                <small>Markera mening och omskriv:</small>
                <button type="button" class="chip selection-rewrite" data-mode="sales">Mer säljande</button>
                <button type="button" class="chip selection-rewrite" data-mode="shorter">Kortare</button>
                <button type="button" class="chip selection-rewrite" data-mode="formal">Mer formellt</button>
                <button type="button" class="chip selection-rewrite" data-mode="clearer">Förtydliga</button>
            </div>
            ${renderQuickPrompts(section.slug)}
            <div class="rewrite-controls">
                <input type="text" class="instruction-input" placeholder="Skriv instruktion för omskrivning">
                <button type="button" class="rewrite-submit" data-slug="${section.slug}">AI omskrivning</button>
                <button type="button" class="save-section" data-slug="${section.slug}">Spara ändring</button>
            </div>
            ${renderHistory(section.slug)}
        `;
        bodyEl.appendChild(wrapper);
    });

    const fullCard = document.createElement('section');
    fullCard.className = 'fullcopy-card';
    fullCard.innerHTML = `
        <header>
            <p class="eyebrow">Annonstryckning</p>
            <h3>Samlad text</h3>
        </header>
        <textarea readonly>${detail.full_copy || detail.sections.map(sec => `${sec.title}\n${sec.content}`).join('\n\n')}</textarea>
    `;
    bodyEl.appendChild(fullCard);
}

function renderStatus(detail, container) {
    const status = detail.status || {};
    const stages = [
        { key: 'data', label: 'Datainsamling' },
        { key: 'vision', label: 'Bildunderlag' },
        { key: 'geodata', label: 'Geodata' },
        { key: 'text', label: 'Text' }
    ];
    container.innerHTML = stages.map(stage => {
        const value = status[stage.key] || 'pending';
        const complete = value === 'completed';
        const symbol = complete ? '✔︎' : value === 'in_progress' ? '…' : '○';
        return `<span class="status-pill ${complete ? 'complete' : ''}">${symbol} ${stage.label}</span>`;
    }).join('');
}

function renderQuickPrompts(slug) {
    const prompts = QUICK_PROMPTS[slug] || [];
    if (!prompts.length) return '';
    return `
        <div class="quick-prompts">
            ${prompts.map(prompt => `<button type="button" class="quick-prompt" data-instruction="${prompt.instruction}">${prompt.label}</button>`).join('')}
        </div>
    `;
}

function renderHistory(slug) {
    const entries = state.current?.section_history?.[slug] || [];
    if (!entries.length) return '';
    return `
        <div class="history-log">
            <details>
                <summary>Historik (${entries.length})</summary>
                ${entries.map((entry, index) => `
                    <div class="history-entry">
                        <header>
                            <span>${new Date(entry.timestamp).toLocaleString('sv-SE')}</span>
                            <span>${entry.source}</span>
                        </header>
                        <p>${entry.title}</p>
                        <button type="button" class="history-restore" data-slug="${slug}" data-index="${index}">Återställ denna version</button>
                    </div>
                `).join('')}
            </details>
        </div>
    `;
}

function buildPayloadFromForm() {
    const rawAdvantages = value('advantages');
    const highlightRaw = value('highlights');

    const form = {
        address: value('address'),
        city: value('city'),
        neighborhood: value('neighborhood'),
        municipality: value('municipality'),
        propertyType: value('property-type'),
        livingArea: numberValue('living-area'),
        rooms: numberValue('rooms'),
        floor: value('floor'),
        builtYear: value('built-year'),
        renovatedYear: value('renovated-year'),
        plan: value('plan'),
        kitchen: value('kitchen'),
        living: value('living'),
        bedroom: value('bedroom'),
        bathroom: value('bathroom'),
        outdoor: value('outdoor'),
        storage: value('storage'),
        parking: value('parking'),
        fee: numberValue('fee'),
        operatingCost: value('operating-cost'),
        price: value('price'),
        associationName: value('association-name'),
        associationEconomy: value('association-economy'),
        renovationsDone: value('renovations-done'),
        renovationsPlanned: value('renovations-planned'),
        commonAreas: value('common-areas'),
        communications: value('communications'),
        service: value('service'),
        schools: value('schools'),
        nature: value('nature'),
        tone: document.getElementById('tone').value,
        targetAudience: value('audience'),
        length: Number(document.getElementById('length').value || 2),
        instructions: value('instructions'),
        advantages: listFromLines(rawAdvantages),
        highlights: listFromLines(highlightRaw),
    };

    const mergedHighlights = Array.from(new Set([...(form.highlights || []), ...(form.advantages || [])]));
    const sections = buildSectionsFromForm({ ...form, highlights: mergedHighlights });
    const instructionSummary = buildInstructionSummary(form);

    return {
        address: form.address,
        tone: form.tone,
        target_audience: form.targetAudience,
        highlights: mergedHighlights,
        fee: form.fee,
        living_area: form.livingArea,
        rooms: form.rooms,
        instructions: instructionSummary,
        sections,
    };
}

async function submitListing(event) {
    event.preventDefault();
    const messageEl = document.getElementById('form-message');
    messageEl.textContent = 'Skickar...';

    const payload = buildPayloadFromForm();
    if (!payload.address) {
        messageEl.textContent = 'Adress krävs för att skapa utkast.';
        return;
    }

    const fileInput = document.getElementById('photo');
    try {
        let res;
        if (fileInput.files.length > 0) {
            const formData = new FormData();
            Object.entries(payload).forEach(([key, val]) => {
                if (val === undefined || val === null) return;
                if (key === 'sections') {
                    formData.append('sections', JSON.stringify(val));
                } else if (Array.isArray(val)) {
                    formData.append(key, val.join(', '));
                } else {
                    formData.append(key, val);
                }
            });
            formData.append('photo', fileInput.files[0]);
            res = await fetch('/api/listings/', { method: 'POST', body: formData });
        } else {
            res = await fetch('/api/listings/', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });
        }

        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || 'Något gick fel');
        }

        messageEl.textContent = 'Utkast sparat';
        event.target.reset();
        await fetchListings();
    } catch (err) {
        messageEl.textContent = err.message;
    }
}

const sidebarToggle = document.getElementById('sidebar-toggle');
if (sidebarToggle) {
    sidebarToggle.addEventListener('click', () => {
        document.body.classList.toggle('sidebar-collapsed');
    });
}

document.getElementById('listing-form').addEventListener('submit', submitListing);
document.getElementById('refresh-btn').addEventListener('click', fetchListings);
document.querySelectorAll('.refresh-inline').forEach(btn => btn.addEventListener('click', fetchListings));

document.getElementById('listing-list').addEventListener('click', event => {
    const item = event.target.closest('.list__item');
    if (item?.dataset.id) {
        selectListing(item.dataset.id);
    }
});

const gridRefresh = document.getElementById('grid-refresh');
if (gridRefresh) {
    gridRefresh.addEventListener('click', fetchListings);
}

const compactToggle = document.getElementById('compact-toggle');
if (compactToggle) {
    compactToggle.addEventListener('click', () => {
        const grid = document.getElementById('listing-grid');
        if (!grid) return;
        grid.classList.toggle('compact');
    });
}

document.getElementById('detail-body').addEventListener('click', event => {
    if (event.target.matches('.rewrite-submit')) {
        const slug = event.target.dataset.slug;
        const container = event.target.closest('.section-editor');
        const instructionInput = container.querySelector('.instruction-input');
        const instruction = instructionInput.value.trim() || DEFAULT_PROMPT;
        rewriteSection(slug, instruction);
    }
    if (event.target.matches('.save-section')) {
        const slug = event.target.dataset.slug;
        const container = event.target.closest('.section-editor');
        const title = container.querySelector('h3').textContent.trim();
        const content = container.querySelector('textarea').value.trim();
        saveSection(slug, title, content);
    }
    if (event.target.matches('.quick-prompt')) {
        const instruction = event.target.dataset.instruction;
        const slug = event.target.closest('.section-editor').dataset.slug;
        rewriteSection(slug, instruction);
    }
    if (event.target.matches('.history-restore')) {
        const slug = event.target.dataset.slug;
        const index = Number(event.target.dataset.index);
        restoreFromHistory(slug, index);
    }
    if (event.target.matches('.delete-section')) {
        const slug = event.target.dataset.slug;
        deleteSection(slug);
    }
    if (event.target.matches('.selection-rewrite')) {
        const mode = event.target.dataset.mode;
        const slug = event.target.closest('.section-editor').dataset.slug;
        rewriteSelection(slug, mode);
    }
});

document.getElementById('copy-text-btn').addEventListener('click', copyFullText);
document.getElementById('copy-html-btn').addEventListener('click', copyHTML);
document.getElementById('download-txt-btn').addEventListener('click', downloadText);
document.getElementById('delete-listing-btn').addEventListener('click', deleteCurrentListing);
document.getElementById('add-section-btn').addEventListener('click', addNewSection);
document.querySelectorAll('[data-rewrite-all]').forEach(btn => {
    btn.addEventListener('click', () => rewriteAllSections(btn.dataset.rewriteAll));
});
const applyToneBtn = document.getElementById('apply-tone-btn');
if (applyToneBtn) {
    applyToneBtn.addEventListener('click', () => {
        const tone = document.getElementById('tone-switcher').value;
        rewriteAllSections('tone', tone);
    });
}

async function rewriteSection(slug, instruction, skipListRefresh = false) {
    if (!state.selectedId) return;
    try {
        const res = await fetch(`/api/listings/${state.selectedId}/sections/${slug}/rewrite`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ instruction })
        });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || 'Misslyckades med omskrivning');
        }
        const fallback = res.headers.get('X-Generator-Fallback') === '1';
        state.current = await res.json();
        renderDetail(state.current);
        if (!skipListRefresh) {
            await fetchListings();
        }
        if (fallback) {
            alert('AI kunde inte skriva om texten just nu, visade istället en enklare justering. Försök igen senare.');
        }
    } catch (err) {
        alert(err.message);
    }
}

async function rewriteAllSections(kind, toneOverride) {
    if (!state.current?.sections?.length) return;
    const instruction = instructionForRewriteKind(kind, toneOverride);
    for (const section of state.current.sections) {
        // eslint-disable-next-line no-await-in-loop
        await rewriteSection(section.slug, instruction, true);
    }
    await fetchListings();
}

function instructionForRewriteKind(kind, toneOverride) {
    switch (kind) {
    case 'short':
        return 'Kondensera sektionen till 2-3 meningar men behåll fakta och rytm.';
    case 'long':
        return 'Utveckla sektionen till en längre version med mer miljö och detaljer, utan att hitta på fakta.';
    case 'sales':
        return 'Gör texten mer säljande, unik och varm men fortfarande trovärdig.';
    case 'formal':
        return 'Justera till en mer formell och faktabaserad ton med tydliga stycken.';
    case 'tone':
        return `Byt till tonen "${toneOverride}" och variera meningstakterna. Bevara alla fakta.`;
    default:
        return DEFAULT_PROMPT;
    }
}

async function saveSection(slug, title, content) {
    if (!state.selectedId || !content) return;
    try {
        const res = await fetch(`/api/listings/${state.selectedId}/sections/${slug}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ title, content })
        });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || 'Misslyckades med att spara');
        }
        state.current = await res.json();
        renderDetail(state.current);
        await fetchListings();
    } catch (err) {
        alert(err.message);
    }
}

function copyFullText() {
    if (!state.current) return;
    const text = getFullCopy(state.current);
    if (!text) return;
    navigator.clipboard.writeText(text).then(() => flashButton('copy-text-btn'));
}

function copyHTML() {
    if (!state.current) return;
    const text = getFullCopy(state.current);
    if (!text) return;
    const html = text.split(/\n\s*\n/).map(par => `<p>${par.replace(/\n/g, ' ')}</p>`).join('\n');
    navigator.clipboard.writeText(html).then(() => flashButton('copy-html-btn'));
}

function downloadText() {
    if (!state.current) return;
    const text = getFullCopy(state.current);
    if (!text) return;
    const blob = new Blob([text], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `${state.current.address || 'listing'}.txt`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
}

function getFullCopy(detail) {
    if (detail.full_copy) return detail.full_copy;
    if (detail.sections?.length) {
        return detail.sections.map(sec => `${sec.title}\n${sec.content}`).join('\n\n');
    }
    return '';
}

function flashButton(id) {
    const btn = document.getElementById(id);
    const original = btn.textContent;
    btn.textContent = 'Kopierat!';
    setTimeout(() => (btn.textContent = original), 1500);
}

function restoreFromHistory(slug, index) {
    const entries = state.current?.section_history?.[slug];
    if (!entries || !entries[index]) return;
    const entry = entries[index];
    saveSection(slug, entry.title, entry.content);
}

function rewriteSelection(slug, mode) {
    const container = document.querySelector(`.section-editor[data-slug="${slug}"]`);
    const textarea = container?.querySelector('textarea');
    if (!textarea) return;
    const selected = textarea.value.substring(textarea.selectionStart, textarea.selectionEnd).trim();
    if (!selected) {
        alert('Markera en mening i texten först.');
        return;
    }
    const modeInstruction = {
        sales: 'gör mer säljande och unik',
        formal: 'gör mer formell och rak',
        shorter: 'förkorta men bevara kärnan',
        clearer: 'förtydliga och ta bort upprepningar',
    }[mode] || 'skriv om med variation';
    const instruction = `Omskriv markerad text: "${selected}" och ${modeInstruction}.`;
    rewriteSection(slug, instruction);
}

async function deleteSection(slug) {
    if (!state.selectedId) return;
    if (!window.confirm('Ta bort denna sektion?')) return;
    try {
        const res = await fetch(`/api/listings/${state.selectedId}/sections/${slug}`, {
            method: 'DELETE'
        });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || 'Misslyckades med att ta bort sektionen');
        }
        state.current = await res.json();
        renderDetail(state.current);
        await fetchListings();
    } catch (err) {
        alert(err.message);
    }
}

function addNewSection() {
    if (!state.selectedId) return;
    const title = window.prompt('Titel för nya sektionen:', 'Ny sektion');
    if (!title) return;
    const slug = slugify(title);
    const content = window.prompt('Förifyll text (valfritt):', `${title} – fyll på beskrivningen här.`) || `${title} – fyll på beskrivningen här.`;
    saveSection(slug, title, content);
}

async function deleteCurrentListing() {
    if (!state.selectedId) return;
    const listing = state.listings.find(l => l.id === state.selectedId);
    const ok = window.confirm(`Ta bort objektet "${listing?.address || ''}"?`);
    if (!ok) return;

    try {
        const res = await fetch(`/api/listings/${state.selectedId}/`, { method: 'DELETE' });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || 'Misslyckades med att ta bort objektet');
        }
        state.selectedId = null;
        resetDetailPanel('Objekt raderat.');
        await fetchListings();
    } catch (err) {
        alert(err.message);
    }
}

function slugify(value) {
    return value
        .toLowerCase()
        .trim()
        .replace(/[^a-z0-9åäö\s-]/g, '')
        .replace(/\s+/g, '-')
        .replace(/-+/g, '-');
}

resetDetailPanel();
fetchListings();

const statusStream = new EventSource('/api/events');
statusStream.addEventListener('status', handleStatusEvent);

function handleStatusEvent(event) {
    try {
        const payload = JSON.parse(event.data);
        if (!payload?.listing_id) return;
        if (payload.status?.data === 'deleted') {
            state.listings = state.listings.filter(item => item.id !== payload.listing_id);
            if (state.selectedId === payload.listing_id) {
                state.selectedId = null;
                resetDetailPanel('Objekt raderat.');
            }
            renderListingList();
            renderListingGrid();
            return;
        }
        const target = state.listings.find(item => item.id === payload.listing_id);
        if (target) {
            target.status = payload.status;
        }
        if (state.current && state.current.id === payload.listing_id) {
            state.current.status = payload.status;
            renderStatus(state.current, document.getElementById('detail-status'));
        }
        renderListingList();
        renderListingGrid();
    } catch (err) {
        console.error('SSE error', err);
    }
}
