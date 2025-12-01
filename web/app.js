async function fetchListings() {
    const listEl = document.getElementById('listing-list');
    listEl.innerHTML = '<li class="list__item">Hämtar...</li>';

    try {
        const res = await fetch('/api/listings/');
        if (!res.ok) throw new Error('Kunde inte hämta listor');
        const data = await res.json();

        if (!data.length) {
            listEl.innerHTML = '<li class="list__item">Inga objekt ännu. Skapa ett första!</li>';
            return;
        }

        listEl.innerHTML = '';
        data.forEach(item => {
            const li = document.createElement('li');
            li.className = 'list__item';
            li.innerHTML = `
                <h3>${item.address}</h3>
                <div class="list__meta">
                    <span class="badge">${item.tone}</span>
                    <span>${item.target_audience}</span>
                    <span>${new Date(item.created_at).toLocaleString('sv-SE')}</span>
                </div>
                ${item.highlights?.length ? `<p>${item.highlights.join(', ')}</p>` : ''}
            `;
            listEl.appendChild(li);
        });
    } catch (err) {
        listEl.innerHTML = `<li class="list__item">${err.message}</li>`;
    }
}

async function submitListing(event) {
    event.preventDefault();
    const messageEl = document.getElementById('form-message');
    messageEl.textContent = 'Skickar...';

    const address = document.getElementById('address').value.trim();
    const tone = document.getElementById('tone').value;
    const audience = document.getElementById('audience').value.trim();
    const highlightsRaw = document.getElementById('highlights').value.trim();
    const highlights = highlightsRaw ? highlightsRaw.split(',').map(h => h.trim()).filter(Boolean) : [];

    try {
        const res = await fetch('/api/listings/', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ address, tone, target_audience: audience, highlights })
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

fetchListings();
