/*

attempt at a combined JS file for local.fm, coming up with
corresponding JS for each widget in the page

main update flow, in the general style of Elm

state -> data -> refresh widgets -> state updates

our state is very simple, really just represents what the
view is displaying without the data (should be stored in the
url hash) but is used to fetch remote data which is displayed
by the widgets

this model only really seems sensible for apps with limited
interactivity - remote data browsers like this one

updates to the state are caused by ui clicks

this triggers remote data to be fetched by a datasource function
which then refreshes all widgets that depend on that data

*/

// Page is a simple container for multiple widgets on a single page
class Page {
    constructor(initialState) {
        this.widgets = []
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
        console.log(this.deps)
        console.log("adding widget:" + w)
        this.widgets.push(w)
    }

    // call each registered datasource function and pass
    // the results to each widget that depends on it
    refreshData() {
        console.log("refreshing data for state:")
        console.log(this.state)

        for (let [fn, widgets] of this.deps) {
            console.log("updating source:")
            console.log(fn)
            console.log(widgets)

            let p = fn(this.state)
            p.then(data => {
                console.log("got data")
                console.log(data)
                for (let w of widgets) {
                    try {
                        w.refresh(this.state, data)
                    } catch (err) {
                        // XXX this is an internal error thrown by a widget?
                        console.log("error refreshing widget:")
                        console.log(w)
                        console.log(err)
                    }
                }
            }).catch(e => {
                // XXX error fetching remote data
                // XXX need to invalidate the widget
                console.log("datasource error")
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

    // xxx call this update instead?
    refresh(state, data) {
        console.log("refreshing datebar")
        console.log(state)
        console.log(data)

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

class ArtistGrid {
    constructor(page) {
        this.page = page
    }
    init() {
        // no event handlers, so nothing necessary
    }
    refresh(state, data) {
        console.log("refreshing ArtistGrid")
        console.log(state)
        console.log(data)

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
        console.log("refreshing TrackList")
        console.log(state)
        console.log(data)

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
        console.log("refreshing ListeningClock")
        console.log(state)
        console.log(data)

        console.log(`got ${data.length} hours from json call`)

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
        console.log("refreshing NewArtists")
        console.log(state)
        console.log(data)

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
        const artistDataUrl = "data/artists"
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
        const monthlyTrackUrl = "/data/monthlyTracks"
        return fetch(monthlyTrackUrl + makeQuery(state))
            .then(response => response.json())
    }

    function topNewArtists(state) {
        const monthlyArtistUrl = "/data/monthlyArtists"
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

    // do the initial data refresh, which will cause the
    // widgets to be updated with newly fetched data
    page.refreshData()
}

/// xxx junk drawer
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
