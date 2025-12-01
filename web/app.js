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
            const imageMarkup = item.image_url ? `<img class="list__image" src="${item.image_url}" alt="Bild för ${item.address}" loading="lazy">` : '';
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
            const factsMarkup = facts.length ? `<div class="list__facts">${facts.join(' · ')}</div>` : '';
            const sectionsMarkup = Array.isArray(item.sections) && item.sections.length
                ? `<div class="section-grid">
                    ${item.sections.map(section => `
                        <article class="section-card">
                            <h4>${section.title}</h4>
                            <p>${section.content || 'Ingen text än.'}</p>
                        </article>
                    `).join('')}
                </div>` : '';
            const pois = item.insights?.geodata?.points_of_interest || [];
            const transit = item.insights?.geodata?.transit || [];
            const poiMarkup = pois.length || transit.length ? `
                <div class="poi-block">
                    <h4>Omgivning</h4>
                    <ul class="poi-list">
                        ${pois.map(p => `<li>${p.name} · ${p.category} · ${p.distance}</li>`).join('')}
                        ${transit.map(t => `<li>${t.mode}: ${t.description}</li>`).join('')}
                    </ul>
                </div>
            ` : '';
            li.innerHTML = `
                <h3>${item.address}</h3>
                <div class="list__meta">
                    <span class="badge">${item.tone}</span>
                    <span>${item.target_audience}</span>
                    <span>${new Date(item.created_at).toLocaleString('sv-SE')}</span>
                </div>
                ${factsMarkup}
                ${item.highlights?.length ? `<p>${item.highlights.join(', ')}</p>` : ''}
                ${sectionsMarkup}
                ${poiMarkup}
                ${imageMarkup}
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

fetchListings();
