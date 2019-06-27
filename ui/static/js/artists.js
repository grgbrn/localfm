const artistDataUrl = "data/artists"

// simple application state
window.state = {
    offset: 0,     // how far back we are from the present
    mode: "month", // current display mode
}

document.addEventListener('DOMContentLoaded', (e) => {

    document.getElementById("prevlink").addEventListener('click', e => {
        window.state.offset += 1
        updateControls(window.state)
        populatePage(window.state)
    })

    document.getElementById("nextlink").addEventListener('click', e => {
        window.state.offset -= 1
        updateControls(window.state)
        populatePage(window.state)
    })

    document.getElementById("daterange").addEventListener('change', e => {

        console.log("date mode was changed:" + e.target.value)
        window.state.mode = e.target.value

        /*
        XXX how to change offset value when switching between modes???

        going from week->month ideally it would display the current month
        going from month->week it would display first month of the week?

        for now, just reset to 0 which isn't great...

        */
       window.state.offset = 0

        updateControls(window.state)
        populatePage(window.state)
    })

    // call with the default value
    updateControls(window.state)
    populatePage(window.state)
})

function updateControls(state) {
    console.log(state)

    // set link labels
    let controlLabel = capitalize(state.mode)
    document.querySelector("#datebar-prev-label").textContent = controlLabel
    document.querySelector("#datebar-next-label").textContent = controlLabel

    // disable "next" link if we're at present time
    if (state.offset == 0) {
        document.getElementById("nextlink").style.visibility = "hidden";
    } else {
        document.getElementById("nextlink").style.visibility = "";
    }
}

function makeQuery(state) {
    return `?mode=${state.mode}&offset=${state.offset}`
}

function populatePage(state) {

    let artistGallery = document.querySelector("div.gallery")

    // populate artist table with results of api call
    let p1 = fetch(artistDataUrl + makeQuery(state))
        .then(response => response.json())
        .then(data => {
            // console.log(`got ${data.length} artists from json call`)
            // console.log(data);

            empty(artistGallery)
            populateArtistGallery(artistGallery, data)
        })
        .catch(error => {
            // XXX what's best practice for catching non-200s?
            console.log("!!! error getting track data")
            console.log(error)
            empty(artistGallery)
            // XXX todo display error in-page
        });
}

function populateArtistGallery(tableDom, artistData) {
    const tmpl = document.querySelector("#artist_tile_template")

    for (const dat of artistData) {
        var clone = document.importNode(tmpl.content, true);
        var div = clone.querySelector("div"); // XXX maybe just use children?
        div.children[0].src = selectCoverImage(dat.urls)
        div.children[1].children[0].textContent = dat.artist;
        div.children[1].children[2].textContent = dat.count;

        tableDom.appendChild(clone);
    }
}

function selectCoverImage(urls) {
    if (!urls || urls.length == 0) {
        return ""
    }
    return randElt(urls)
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

function capitalize(s) {
    return s && s[0].toUpperCase() + s.slice(1);
}