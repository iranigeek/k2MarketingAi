const QUICK_PROMPTS = {
    intro: [
        { label: 'Kortare', instruction: 'Förkorta inledningen men behåll det emotionella tonläget.' },
        { label: 'Premium', instruction: 'Ge texten en mer exklusiv ton och lyft fram unika kvaliteter.' }
    ],
    hall: [
        { label: 'Förvaring', instruction: 'Beskriv hallens förvaring mer ingående och praktiskt.' },
        { label: 'Välkomnande', instruction: 'Fokusera på känslan av att komma hem och ljuset i hallen.' }
    ],
    kitchen: [
        { label: 'Matlagning', instruction: 'Betona kökets funktioner för den som gillar att laga mat.' },
        { label: 'Socialt', instruction: 'Beskriv hur köket öppnar upp för sociala middagar.' }
    ],
    living: [
        { label: 'Familj', instruction: 'Gör texten mer familjär och betona plats för umgänge.' },
        { label: 'Design', instruction: 'Lyft fram design, ljus och material i vardagsrummet.' }
    ],
    area: [
        { label: 'Barnfamilj', instruction: 'Betona skolor, parker och trygga gårdar i området.' },
        { label: 'Citypuls', instruction: 'Fokusera på kommunikationer och restauranger i närområdet.' }
    ]
};

const DEFAULT_PROMPT = 'Polera texten och gör den mer säljande utan att ändra fakta.';

const state = {
    listings: [],
    current: null,
    selectedId: null
};

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
    listEl.innerHTML = '<li class="list__item">Hämtar...</li>';

    try {
        const res = await fetch('/api/listings/');
        if (!res.ok) throw new Error('Kunde inte hämta listor');
        state.listings = await res.json();
        renderListingList();

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
        listEl.innerHTML = `<li class="list__item">${err.message}</li>`;
    }
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

async function submitListing(event) {
    event.preventDefault();
    const messageEl = document.getElementById('form-message');
    messageEl.textContent = 'Skickar...';

    const address = document.getElementById('address').value.trim();
    const tone = document.getElementById('tone').value;
    const audience = document.getElementById('audience').value.trim();
    const highlightsRaw = document.getElementById('highlights').value.trim();
    const fileInput = document.getElementById('photo');
    const fee = document.getElementById('fee').value.trim();
    const livingArea = document.getElementById('living-area').value.trim();
    const rooms = document.getElementById('rooms').value.trim();
    const instructions = document.getElementById('instructions').value.trim();

    try {
        const formData = new FormData();
        formData.append('address', address);
        formData.append('tone', tone);
        formData.append('target_audience', audience);
        formData.append('highlights', highlightsRaw);
        formData.append('fee', fee);
        formData.append('living_area', livingArea);
        formData.append('rooms', rooms);
        formData.append('instructions', instructions);
        if (fileInput.files.length > 0) {
            formData.append('photo', fileInput.files[0]);
        }

        const res = await fetch('/api/listings/', {
            method: 'POST',
            body: formData
        });

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

document.getElementById('listing-form').addEventListener('submit', submitListing);
document.getElementById('refresh-btn').addEventListener('click', fetchListings);

document.getElementById('listing-list').addEventListener('click', event => {
    const item = event.target.closest('.list__item');
    if (item?.dataset.id) {
        selectListing(item.dataset.id);
    }
});

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
});

document.getElementById('copy-text-btn').addEventListener('click', copyFullText);
document.getElementById('copy-html-btn').addEventListener('click', copyHTML);
document.getElementById('download-txt-btn').addEventListener('click', downloadText);
document.getElementById('delete-listing-btn').addEventListener('click', deleteCurrentListing);
document.getElementById('add-section-btn').addEventListener('click', addNewSection);

async function rewriteSection(slug, instruction) {
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
        await fetchListings();
        if (fallback) {
            alert('AI kunde inte skriva om texten just nu, visade istället en enklare justering. Försök igen senare.');
        }
    } catch (err) {
        alert(err.message);
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
    } catch (err) {
        console.error('SSE error', err);
    }
}
