/*

attempt at a combined JS file for local.fm, coming up with
corresponding JS for each component in the page

*/

// simple application state
// XXX figure out where this lives... probably in the Page?
window.state = {
    offset: 0,     // how far back we are from the present
    mode: "month", // current display mode
}


// Page is a simple container for multiple widgets on a single page
class Page {
    constructor() {
        self.widgets = []
    }

    addWidget(w) {
        console.log("adding widget:" + w)
        self.widgets.push(w)
    }

    updateState(newState) {
        self.widgets.forEach(widget => {
            // XXX should this return a promise or something?
            widget.refresh(newState)
        });
    }
}

class DateBar {
    constructor(page) {
        this.page = page
    }

    init() {
        console.log("init DateBar")
        document.getElementById("prevlink").addEventListener('click', e => {
            // XXX don't mutate global state here
            // XXX don't even use global state here!
            window.state.offset += 1
            this.page.updateState(window.state)
        })

        document.getElementById("nextlink").addEventListener('click', e => {
            // XXX don't mutate global state here
            // XXX don't even use global state here!
            window.state.offset += 1
            this.page.updateState(window.state)
        })

        document.getElementById("daterange").addEventListener('change', e => {

            // XXSTATE
            console.log("date mode was changed:" + e.target.value)
            window.state.mode = e.target.value
            /*
            XXX how to change offset value when switching between modes???

            going from week->month ideally it would display the current month
            going from month->week it would display first month of the week?

            for now, just reset to 0 which isn't great...

            */
            window.state.offset = 0
            console.log(this)

            this.page.updateState(window.state)
        })
    }

    refresh(state) {
        console.log("refreshing datebar with state:")
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
}

class ArtistGrid {
    constructor(page) {
        this.page = page
    }
    init() {
        // no event handlers, so nothing necessary
    }
    refresh(state) {
        let artistGallery = document.querySelector("div.gallery")

        // populate artist table with results of api call
        let p1 = fetch(artistDataUrl + makeQuery(state))
            .then(response => response.json())
            .then(data => {
                console.log(data);

                // xxx this is kind of like state update?
                // YYUPDATE(maybe)
                this.updateDataTitle(data)
                empty(artistGallery)
                this.populateArtistGallery(artistGallery, data.artists)
            })
            .catch(error => {
                // XXX what's best practice for catching non-200s?
                // YYUPDATE(maybe)
                console.log("!!! error getting track data")
                console.log(error)
                empty(artistGallery)
            });
    }

    // internal methods
    populateArtistGallery(tableDom, artistData) {
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

    // XXX this one is problematic because it updates the DateBar
    updateDataTitle(data) {
        let label = ""

        let date = new Date(data.startDate)
        if (data.mode == "week") {
            label = `${data.startDate} to ${data.endDate}`
        } else if (data.mode == "month") {
            const monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

            label = `${monthNames[date.getMonth()]} ${date.getFullYear()}`
        } else if (data.mode == "year") {
            label = `${date.getFullYear()}`
        }

        let elt = document.getElementById("datebar-title-label")
        elt.textContent = label
    }
}



/// xxx junk drawer
const artistDataUrl = "data/artists"

function makeQuery(state) {
    return `?mode=${state.mode}&offset=${state.offset}`
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

// specific init for artists page
document.addEventListener('DOMContentLoaded', (e) => {
    console.log("artist page init")
    let page = new Page()

    let db = new DateBar(page)
    db.init()
    page.addWidget(db)

    let ag = new ArtistGrid()
    ag.init()
    page.addWidget(ag)

    page.updateState(window.state)
})