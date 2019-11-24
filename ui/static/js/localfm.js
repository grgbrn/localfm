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
        this.defaultState = initialState
        this.debugLogging = false

        // simple list of widgets (XXX maybe unused?)
        this.widgets = []

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
        console.log("activating debug logging")
        this.debugLogging = true
        this.log(`Page has ${this.widgets.length} widgets, ${this.deps.size} datasources:`)
        for (let [fn, widgets] of this.deps) {
            let widgetNames = widgets.map(widgetName)
            this.log(`  ${fn.name}() -> ${widgetNames.join(' ')}`)
        }
    }

    log(...args) {
        if (this.debugLogging) {
            console.log(...args)
        }
    }

    // call each registered datasource function and pass
    // the results to each widget that depends on it
    // returns a promise that resolves when the refresh is finished
    refreshData() {
        let state = this.getState()
        this.log("refreshing data with state: " + JSON.stringify(state))

        let datasourcePromises = []

        for (let [fn, widgets] of this.deps) {
            this.log("calling datasource: " + fn.name)

            let p = fn(state)
            .then(data => {
                this.log(`${fn.name} got data:`)
                this.log(data)
                for (let w of widgets) {
                    try {
                        this.log("refreshing widget " + widgetName(w))
                        w.refresh(state, data)
                    } catch (err) {
                        // XXX this is an internal error thrown by a widget?
                        console.log("error refreshing widget" + widgetName(w))
                        console.log(err)
                    }
                }
            }).catch(e => {
                console.log("datasource error: " + fn.name)
                console.log(e)
                for (let w of widgets) {
                    try {
                        w.error("Error getting data")
                    } catch (updateErr) {
                        // internal error thrown by the widget
                        console.log("error refreshing widget" + widgetName(w))
                        console.log(updateErr)
                    }
                }
            })
            datasourcePromises.push(p)
        }
        return Promise.all(datasourcePromises)
    }

    getState() {
        let hash = window.location.hash
        if (hash == "" || hash == "#") {
            return this.defaultState
        }
        // trim any leading hash char
        if (hash[0] == "#") {
            hash = hash.substring(1)
        }
        // break down the kv pairs
        let res = {}
        let params = new URLSearchParams(hash)
        for (let [k, v] of params.entries()) {
            res[k] = v
        }
        return res
    }

    // update keys in the state. any key not passed will
    // be left unchanged
    updateState(newState) {
        let s = this.getState()
        for (let [key, val] of Object.entries(newState)) {
            this.log(`updating ${key} = ${val}`)
            s[key] = val
        }
        // maybe easier way to generate the query string?
        let params = new URLSearchParams()
        for (let [k,v] of Object.entries(s)) {
            params.set(k,v)
        }
        window.location.hash = params.toString()

        // call all datasources with new state
        this.refreshData()
    }
}

// can also be used with the "nextbar" template if you don't need
// the date range selector and only want prev/next links
class DateBar {
    constructor(page) {
        this.page = page
        this.hasDateRange = false

        document.getElementById("prevlink").addEventListener('click', e => {
            e.preventDefault()
            let current = this.page.getState()
            this.page.updateState({
                'offset': intOrThrow(current.offset) + 1
            })
        })

        document.getElementById("nextlink").addEventListener('click', e => {
            e.preventDefault()
            let current = this.page.getState()
            this.page.updateState({
                'offset': intOrThrow(current.offset) - 1
            })
        })

        let dateRange = document.getElementById("daterange")
        if (dateRange) {
            this.hasDateRange = true

            dateRange.addEventListener('change', e => {

                console.log("date mode changed:" + e.target.value)
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
        } // end hasDateRange
    }

    // this is necessary to update the labels in the datebar and to
    // disable the "next" link when offset == 0
    refresh(state, data) {
        // set link labels (only for datebar)
        if (this.hasDateRange) {
            let controlLabel = capitalize(state.mode)
            document.querySelector("#datebar-prev-label").textContent = controlLabel
            document.querySelector("#datebar-next-label").textContent = controlLabel

            this.updateTitle(data)
        }

        // disable "next" link if we're at present time
        if (state.offset == 0) {
            document.getElementById("nextlink").style.visibility = "hidden";
        } else {
            document.getElementById("nextlink").style.visibility = "";
        }

    }

    error() { } // don't do anything

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
        this.tableDom = document.querySelector("div.gallery")
    }

    refresh(state, data) {
        empty(this.tableDom)
        if (data.artists && data.artists.length > 0) {
            this.populateArtistGallery(data.artists)
        } else {
            this.message("No data")
        }
    }

    // internal methods
    populateArtistGallery(artistData) {
        const tmpl = document.querySelector("#artist_tile_template")

        for (const dat of artistData) {
            var clone = document.importNode(tmpl.content, true);
            var div = clone.querySelector("div"); // XXX maybe just use children?
            div.children[0].src = selectCoverImage(dat.urls)
            div.children[1].children[0].textContent = dat.artist;
            div.children[1].children[2].textContent = dat.count;

            this.tableDom.appendChild(clone);
        }
    }

    message(message) {
        this.tableDom.innerHTML = "<div>&nbsp;"+message+"</div>"
    }

    error(message) {
        this.tableDom.innerHTML = "<div>&nbsp;<span class='errortext'>"+message+"</span></div>"
    }
}

// xxx more specifically, this is for popular tracks
class TrackList {
    constructor(page) {
        this.page = page
        this.tableDom = document.querySelector("table.listview")
    }

    refresh(state, data) {
        empty(this.tableDom)
        if (data.tracks && data.tracks.length > 0) {
            this.populateTrackList(data.tracks)
        } else {
            this.message("No data")
        }
    }

    // internal
    populateTrackList(trackData) {
        const tmpl = document.querySelector("#trackrow_template")

        for (const dat of trackData) {
            var clone = document.importNode(tmpl.content, true);
            var td = clone.querySelectorAll("td");
            td[0].textContent = dat.rank;
            td[1].children[0].src = randElt(dat.urls);
            td[2].children[0].textContent = dat.title;
            td[2].children[2].textContent = dat.artist;
            td[3].textContent = dat.count;

            this.tableDom.appendChild(clone);
        }
    }
    // display a message in the table instead of data
    message(message) {
        // XXX how slow is innerhtml vs. templating?
        this.tableDom.innerHTML = "<tbody><tr><td>&nbsp;" + message + "</td></tr></tbody>";
    }

    error(message) {
        this.tableDom.innerHTML = "<tbody><tr><td class='errortext'>&nbsp;" + message + "</td></tr></tbody>";
    }

}

// xxx how to factor this a bit better?
class RecentTrackList {
    constructor(page) {
        this.page = page
        this.tableDom = document.querySelector("table.listview")
    }

    refresh(state, data) {
        empty(this.tableDom)
        // xxx response attribute (.activity) needs to be a param
        if (data.activity && data.activity.length > 0) {
            this.populateTrackList(data.activity)
        } else {
            this.message("No data")
        }
    }

    // internal
    populateTrackList(trackData) {
        const tmpl = document.querySelector("#recent_template")

        // initial header needs to reflect the date of the first track
        let lastLabel = this.dateLabel(trackData[0].when)
        let headerRow = this.makeHeaderRow(lastLabel)
        this.tableDom.appendChild(headerRow)

        for (const dat of trackData) {
            // see if a new header row needs to be inserted before
            // this row
            let label = this.dateLabel(dat.when)
            if (label != lastLabel) {
                headerRow = this.makeHeaderRow(label)
                this.tableDom.appendChild(headerRow)
                lastLabel = label
            }

            var clone = document.importNode(tmpl.content, true);
            var td = clone.querySelectorAll("td"); // ???
            td[0].children[0].src = randElt(dat.urls);
            td[1].children[0].textContent = dat.title;
            td[1].children[2].textContent = dat.artist;
            td[2].textContent = this.prettyDate(dat.when);

            // display local timezone-specific text repr in a tooltip
            let d = new Date(dat.when)
            td[2].setAttribute('title', d.toLocaleString());

            this.tableDom.appendChild(clone);
        }
    }

    // return an array of [secs, days] of how old this time string is
    dateDiffs(date) {
        let diff = (((new Date()).getTime() - date.getTime()) / 1000)
        let day_diff = Math.floor(diff / 86400)

        if (isNaN(day_diff) || day_diff < 0 || day_diff >= 31) {
            throw `can't calculate date diffs for: ${date}`
        }
        return [diff, day_diff]
    }

    dateLabel(time) {
        let date = new Date(time)
        let [secs, days] = this.dateDiffs(date)
        if (days == 0) {
            return "Today"
        } else if (days == 1) {
            return "Yesterday"
        } else {
            return date.toDateString()
        }
    }

    // Takes an ISO time and returns a string representing how
    // long ago the date represents.
    // https://johnresig.com/blog/javascript-pretty-date/
    prettyDate(time) {
        // var date = new Date((time || "").replace(/-/g, "/").replace(/[TZ]/g, " ")),
        var date = new Date(time),
            diff = (((new Date()).getTime() - date.getTime()) / 1000),
            day_diff = Math.floor(diff / 86400);

        if (isNaN(day_diff) || day_diff < 0 || day_diff >= 31) return;

        return day_diff == 0 && (
            diff < 60 && "just now" ||
            diff < 120 && "1 minute ago" ||
            diff < 3600 && Math.floor(diff / 60) + " minutes ago" ||
            diff < 7200 && "1 hour ago" ||
            diff < 86400 && Math.floor(diff / 3600) + " hours ago") ||
            day_diff >= 1 && date.toLocaleTimeString()
    }

    makeHeaderRow(label) {
        const headerTmpl = document.querySelector("#date_title")
        var clone = document.importNode(headerTmpl.content, true);
        var td = clone.querySelectorAll("td"); // skip past the tr
        td[0].textContent = label;
        return clone
    }

    // display a message in the table instead of data
    message(message) {
        // XXX how slow is innerhtml vs. templating?
        this.tableDom.innerHTML = "<tbody><tr><td>&nbsp;" + message + "</td></tr></tbody>";
    }

    error(message) {
        this.tableDom.innerHTML = "<tbody><tr><td class='errortext'>&nbsp;" + message + "</td></tr></tbody>";
    }
}


class ListeningClock {
    constructor(page) {
        this.page = page
        this.chartContainer = document.querySelector('.chartcontainer')
    }
    refresh(state, data) {
        // wipe out the existing canvas chart and recreate it from scratch
        empty(this.chartContainer)
        this.chartContainer.innerHTML = "<canvas id='myChart'></canvas>"

        if (data.clock && data.clock.length > 0) {
            let currentValues = data.clock.map(x => x.count)
            let averageValues = data.clock.map(x => x.avgCount)

            let graphTitle = capitalize(data.mode) + "ly Listening Times"

            this.populateListeningClock(
                graphTitle,
                `6 ${data.mode} avg`,
                currentValues,
                averageValues);

        }
    }

    // internal
    populateListeningClock(title, avgLabel, currentValues, averageValues) {
        // construct a list of 2-digit strings 00-23
        let labels = [...Array(24).keys()].map(x => {
            let s = String(x);
            if (s.length == 1) {
                s = `0${s}`
            };
            return s
        });

        let chartDom = document.getElementById('myChart');
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
                    label: avgLabel,
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
        this.tableDom = document.querySelector("table.tinylist")
    }
    refresh(state, data) {
        empty(this.tableDom)
        if (data.artists && data.artists.length > 0) {
            this.populateArtistList(data.artists);
        } else {
            this.message("No data")
        }
    }

    populateArtistList(artistData) {
        const tmpl = document.querySelector("#artistrow_template")

        for (const dat of artistData) {
            var clone = document.importNode(tmpl.content, true);
            var td = clone.querySelectorAll("td");
            // XXX this style of child ref may be too fragile
            td[0].children[0].src = randElt(dat.urls);
            td[1].children[0].textContent = dat.artist;
            td[1].children[2].textContent = dat.count;

            this.tableDom.appendChild(clone);
        }
    }

    message(message) {
        // XXX how slow is innerhtml vs. templating?
        this.tableDom.innerHTML = "<tbody><tr><td>&nbsp;" + message + "</td></tr></tbody>";
    }

    error(message) {
        this.tableDom.innerHTML = "<tbody><tr><td class='errortext'>&nbsp;" + message + "</td></tr></tbody>";
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
    let db = new DateBar(page)
    page.addWidget(db, [topArtists])

    let ag = new ArtistGrid()
    page.addWidget(ag, [topArtists])

    //page.debugDeps()

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
    page.addWidget(db, [topTracks])

    let tracks = new TrackList(page)
    page.addWidget(tracks, [topTracks])

    let artists = new NewArtists(page)
    page.addWidget(artists, [topNewArtists])

    let clock = new ListeningClock(page)
    page.addWidget(clock, [listeningClock])

    //page.debugDeps()

    // do the initial data refresh, which will cause the
    // widgets to be updated with newly fetched data
    page.refreshData()
}

function initRecentPage() {
    // init new page with initial state
    let page = new Page({
        offset: 0,     // how far back we are from the present
        count: 20,     // number of tracks to display
    })

    // define data sources that retrieve external data based
    // on that state
    // must be a function that retuns a promise / async fn
    function recentTracks(state) {
        const artistDataUrl = "data/recentTracks"
        return fetch(artistDataUrl + makeQuery(state))
            .then(response => response.json())
    }

    let db = new DateBar(page)
    page.addWidget(db, [recentTracks])

    let tracks = new RecentTrackList(page)
    page.addWidget(tracks, [recentTracks])

    // do the initial data refresh, which will cause the
    // widgets to be updated with newly fetched data
    page.refreshData().then(_ => {
        // start websocket connection
        console.log("starting websocket connection...")
        let ws = new WebSocket('ws://' + window.location.host + '/ws');

        ws.addEventListener('open', e => {
            console.log("websocket connection opened")
        })
        ws.addEventListener('close', e => {
            // XXX flesh out close behavior
            // XXX should wait a bit and try to reconnect
            console.log("websocket connection closed")
        })
        ws.addEventListener('message', function(e) {
            console.log("got a message from the server:")
            var msg = JSON.parse(e.data)
            console.log(msg)
            console.log("refreshing recent tracks")
            page.refreshData()
        })
        ws.addEventListener('error', e => {
            // XXX flesh out error behavior
            console.log("websocket error!")
        })

        // XXX this should really be attached to a button
        // XXX also need a a whoami call to get logged-in identity
        window.refreshTracks = function() {
            ws.send(JSON.stringify({
                        username: "grgbrn",
                        message: "refresh"
                    }
            ));
        }
    })

}

/// xxx junk drawer

function makeQuery(state) {
    let buf = ""
    for (let [key, value] of Object.entries(state)) {
        if (buf.length > 0) {
            buf += "&"
        }
        buf += `${key}=${value}`
    }
    let tzname = Intl.DateTimeFormat().resolvedOptions().timeZone
    tzname = encodeURIComponent(tzname)
    return `?${buf}&tz=${tzname}`
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

// parse an int from a string or throw an exception
function intOrThrow(value) {
    if (/^[-+]?(\d+)$/.test(value)) {
      return Number(value);
    } else {
      throw "can't parse int param"
    }
  }
