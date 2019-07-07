/*

Combined JS file for local.fm that defines widgets
and datasources and allows them to be combined in a page

Page maintains a simple notion of state - for this app, it
simply represents what the page is displaying, but not the
data itself (e.g. "popular artists for June 2019" but not
the actual ranked list of artists)

Datasources are simply functions that query the remote
datasource to get the data described by a state

Widgets are classes that handle the display of the data
from the Datasources, and handling events that cause the
state to be updated.

The main update flow is in the general style of Elm:

State -> Datasources -> Widgets -> state updates

The Page maintains simple dependencies of which datasources
are used to refresh wich widgets. Currently each widget can
depend on only a single datasource, but a single datasource
can "feed" multiple widgets. This model only really works
for apps with limited interactivity, things like remote data
browsers

UI events make changes to the state and then refresh all
datasources. When the new data is received, all dependent
widgets are refreshed

Current limitations:

- each ui event causes all datasources to be refreshed
- each widget can only depend on a single datasource

*/

// Page is a simple container for multiple widgets on a single page
class Page {
    constructor(initialState) {
        this.widgets = [] // XXX what do we use this for?
        this.state = initialState

        // holds a map of DataSources -> widgets
        // XXX wouldn't typescript be nice here
        this.deps = new Map()
    }

    addWidget(w, dataDeps) {
        // store datasource -> [widget...]
        // in the page map of data dependencies
        for (let dep of dataDeps) {
            if (!this.deps.has(dep)) {
                this.deps.set(dep, [])
            }
            this.deps.get(dep).push(w)
        }
        this.widgets.push(w)
    }

    debugDeps() {
        console.log(`Page has ${this.widgets.length} widgets, ${this.deps.size} datasources:`)
        for (let [fn, widgets] of this.deps) {
            let widgetNames = widgets.map(widgetName)
            console.log(`  ${fn.name}() -> ${widgetNames.join(' ')}`)
        }
    }

    // call each registered datasource function and pass
    // the results to each widget that depends on it
    refreshData() {
        console.log("refreshing data with state: " + JSON.stringify(this.state))

        for (let [fn, widgets] of this.deps) {
            console.log("calling datasource: " + fn.name)

            let p = fn(this.state)
            p.then(data => {
                console.log(`${fn.name} got data:`)
                console.log(data)
                for (let w of widgets) {
                    try {
                        console.log("refreshing widget " + widgetName(w))
                        w.refresh(this.state, data)
                    } catch (err) {
                        // XXX this is an internal error thrown by a widget?
                        console.log("error refreshing widget" + widgetName(w))
                        console.log(err)
                    }
                }
            }).catch(e => {
                // XXX error fetching remote data
                // XXX need to invalidate the widgets
                console.log("datasource error: " + fn.name)
                console.log(e)
            })
        }
    }

    // update keys in the state. any key not passed will
    // be left unchanged
    updateState(newState) {
        for (let [key, val] of Object.entries(newState)) {
            console.log(`updating ${key} = ${val}`)
            this.state[key] = val
        }

        // call all datasources with new state
        this.refreshData()
    }
}

class DateBar {
    constructor(page) {
        this.page = page
    }

    init() {
        document.getElementById("prevlink").addEventListener('click', e => {
            this.page.updateState({
                'offset': this.page.state.offset + 1
            })
        })

        document.getElementById("nextlink").addEventListener('click', e => {
            this.page.updateState({
                'offset': this.page.state.offset - 1
            })
        })

        document.getElementById("daterange").addEventListener('change', e => {

            console.log("date mode was changed:" + e.target.value)
            /*
            XXX how to change offset value when switching between modes???

            going from week->month ideally it would display the current month
            going from month->week it would display first month of the week?

            for now, just reset to 0 which isn't great...
            */
            this.page.updateState({
                'mode': e.target.value,
                'offset': 0
            })
        })
    }

    // this is necessary only to update the labels in the datebar
    refresh(state, data) {
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

        this.updateTitle(data)
    }

    updateTitle(data) {
        let label = ""

        let startDate = new Date(data.startDate)
        let endDate = new Date(data.endDate)
        if (data.mode == "week") {
            label = `${startDate.toDateString()} to ${endDate.toDateString()}`
        } else if (data.mode == "month") {
            const monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

            label = `${monthNames[startDate.getMonth()]} ${startDate.getFullYear()}`
        } else if (data.mode == "year") {
            label = `${startDate.getFullYear()}`
        }

        let elt = document.getElementById("datebar-title-label")
        elt.textContent = label
    }
}

class ArtistGrid {
    constructor(page) {
        this.page = page
    }
    init() {
        // no event handlers, so nothing necessary
    }
    refresh(state, data) {
        let artistGallery = document.querySelector("div.gallery")
        empty(artistGallery)
        this.populateArtistGallery(artistGallery, data.artists)

        // XXX this belongs in an error handler
        // .catch(error => {
        //     // XXX what's best practice for catching non-200s?
        //     // YYUPDATE(maybe)
        //     console.log("!!! error getting track data")
        //     console.log(error)
        //     empty(artistGallery)
        // });
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
}

class TrackList {
    constructor(page) {
        this.page = page
    }
    init() {
        // no event handlers, so nothing necessary
    }
    refresh(state, data) {
        var trackListTable = document.querySelector("table.listview")
        empty(trackListTable)
        this.populateTrackList(trackListTable, data.tracks)
    }

    // internal
    populateTrackList(tableDom, trackData) {
        const tmpl = document.querySelector("#trackrow_template")

        for (const dat of trackData) {
            var clone = document.importNode(tmpl.content, true);
            var td = clone.querySelectorAll("td");
            td[0].textContent = dat.rank;
            td[1].children[0].src = randElt(dat.urls);
            td[2].children[0].textContent = dat.title;
            td[2].children[2].textContent = dat.artist;
            td[3].textContent = dat.count;

            tableDom.appendChild(clone);
        }
    }
}

class ListeningClock {
    constructor(page) {
        this.page = page
    }
    refresh(state, data) {
        var ctx = document.getElementById('myChart');
        let currentValues = data.map(x => x.count)
        let averageValues = data.map(x => x.avgCount)

        this.populateListeningClock(ctx,
            'Apr 2019 Listening Clock', // XXX
            currentValues,
            averageValues);
    }

    // internal
    populateListeningClock(chartDom, title, currentValues, averageValues) {
        // construct a list of 2-digit strings 00-23
        let labels = [...Array(24).keys()].map(x => {
            let s = String(x);
            if (s.length == 1) {
                s = `0${s}`
            };
            return s
        });

        var myChart = new Chart(chartDom, {
            type: 'line',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Scrobbles',
                    data: currentValues,
                    backgroundColor: 'rgba(0,0,255,0.6)',
                    borderColor: 'blue',
                }, {
                    label: '6 Month Avg',
                    data: averageValues,
                }]
            },
            options: {
                responsive: true,
                title: {
                    display: true,
                    text: title
                },
                tooltips: {
                    mode: 'index',
                    intersect: false,
                },
                hover: {
                    mode: 'nearest',
                    intersect: true
                },
                scales: {
                    xAxes: [{
                        display: true,
                        scaleLabel: {
                            display: true,
                            labelString: 'Hour'
                        }
                    }],
                    yAxes: [{
                        display: false,
                        scaleLabel: {
                            display: true,
                            labelString: 'Value'
                        }
                    }]
                }
            }
        });
    }
}

class NewArtists {
    constructor(page) {
        this.page = page
    }
    refresh(state, data) {
        var artistListTable = document.querySelector("table.tinylist")
        empty(artistListTable)
        this.populateArtistList(artistListTable, data.artists);
    }

    populateArtistList(tableDom, artistData) {
        const tmpl = document.querySelector("#artistrow_template")

        for (const dat of artistData) {
            var clone = document.importNode(tmpl.content, true);
            var td = clone.querySelectorAll("td");
            // XXX this style of child ref may be too fragile
            td[0].children[0].src = randElt(dat.urls);
            td[1].children[0].textContent = dat.artist;
            td[1].children[2].textContent = dat.count;

            tableDom.appendChild(clone);
        }
    }
}

// page-specific initializers
function initArtistPage() {
    // init new page with initial state
    let page = new Page({
        offset: 0,     // how far back we are from the present
        mode: "month", // current display mode
    })

    // define data sources that retrieve external data based
    // on that state
    // must be a function that retuns a promise / async fn
    function topArtists(state) {
        const artistDataUrl = "data/topArtists"
        return fetch(artistDataUrl + makeQuery(state))
            .then(response => response.json())
    }

    // define widgets that depend on those data sources
    // XXX this is a bit too verbose
    let db = new DateBar(page)
    db.init()
    page.addWidget(db, [topArtists])

    let ag = new ArtistGrid()
    ag.init()
    page.addWidget(ag, [topArtists])

    page.debugDeps()

    // do the initial data refresh, which will cause the
    // widgets to be updated with newly fetched data
    page.refreshData()
}

function initMonthlyPage() {
    // init new page with initial state
    let page = new Page({
        offset: 0,     // how far back we are from the present
        mode: "month", // current display mode
    })

    // define data sources
    function topTracks(state) {
        const monthlyTrackUrl = "/data/topTracks"
        return fetch(monthlyTrackUrl + makeQuery(state))
            .then(response => response.json())
    }

    function topNewArtists(state) {
        const monthlyArtistUrl = "/data/topNewArtists"
        return fetch(monthlyArtistUrl + makeQuery(state))
            .then(response => response.json())
    }

    function listeningClock(state) {
        const listeningClockUrl = "/data/listeningClock"
        return fetch(listeningClockUrl + makeQuery(state))
            .then(response => response.json())
    }

    // define widgets
    let db = new DateBar(page)
    db.init()
    page.addWidget(db, [topTracks])

    let tracks = new TrackList(page)
    tracks.init()
    page.addWidget(tracks, [topTracks])

    let artists = new NewArtists(page)
    page.addWidget(artists, [topNewArtists])

    let clock = new ListeningClock(page)
    page.addWidget(clock, [listeningClock])

    page.debugDeps()

    // do the initial data refresh, which will cause the
    // widgets to be updated with newly fetched data
    page.refreshData()
}

/// xxx junk drawer

function makeQuery(state) {
    let tzname = Intl.DateTimeFormat().resolvedOptions().timeZone
    tzname = encodeURIComponent(tzname)
    return `?mode=${state.mode}&offset=${state.offset}&tz=${tzname}`
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

function widgetName(w) {
    return `[${w.constructor.name}]`
}
