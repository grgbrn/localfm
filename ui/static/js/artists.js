document.addEventListener('DOMContentLoaded', (e) => {

    document.getElementById("daterange").addEventListener('change', e => {
        console.log("date range was changed:" + e.target.value)
        populatePage(e.target.value);
    })

    populatePage('7d')
})

// since there's only one set of data available for now, cheat
// and just reverse the order of loaded data on each change
// to simulate different data being returned from server
window.populatePageCount = 0;

function populatePage(timePeriod) {

    // populate artist table with results of api call
    let p1 = fetch('data/artists.json')
        .then(response => response.json())
        .then(data => {
            console.log(`got ${data.length} artists from json call`)

            if (window.populatePageCount % 2 == 1) {
                data = data.reverse();
            }

            console.log(data);

            var artistGallery = document.querySelector("div.gallery")
            empty(artistGallery)
            populateArtistGallery(artistGallery, data)

            window.populatePageCount += 1
        })
        .catch(error => {
            console.log("!!! error getting track data")
            console.log(error)
        });
}

function populateArtistGallery(tableDom, artistData) {
    const tmpl = document.querySelector("#artist_tile_template")

    for (const dat of artistData) {
        var clone = document.importNode(tmpl.content, true);
        var div = clone.querySelector("div"); // XXX maybe just use children?
        div.children[0].src = randElt(dat.urls)
        div.children[1].children[0].textContent = dat.artist;
        div.children[1].children[2].textContent = dat.count;

        tableDom.appendChild(clone);
    }
}

// jquery-like helpers
function empty(domElt) {
    while (domElt.firstChild) {
        domElt.removeChild(domElt.firstChild);
    }
}

function randElt(arr) {
    return arr[Math.floor(Math.random() * arr.length)];
}