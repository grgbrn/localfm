const monthlyTrackUrl = "/data/monthlyTracks"
const monthlyArtistUrl = "/data/monthlyArtists"
const listeningClockUrl = "/data/listeningClock"

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

    // populate track listview with results of api call
    let p1 = fetch(monthlyTrackUrl)
        .then(response => response.json())
        .then(data => {
            console.log(`got ${data.length} tracks from json call`)

            if (window.populatePageCount % 2 == 1) {
                data = data.reverse();
            }

            var trackListTable = document.querySelector("table.listview")
            empty(trackListTable)
            populateTrackList(trackListTable, data)
        })
        .catch(error => {
            console.log("!!! error getting track data")
            console.log(error)
        });

    // populate new artist listview from api call
    let p2 = fetch(monthlyArtistUrl)
        .then(response => response.json())
        .then(data => {
            console.log(`got ${data.length} artists from json call`)

            if (window.populatePageCount % 2 == 1) {
                data = data.reverse();
            }

            var artistListTable = document.querySelector("table.tinylist")
            empty(artistListTable)
            populateArtistList(artistListTable, data);
        })
        .catch(error => {
            console.log("!!! error getting track data")
            console.log(error)
        });

    // populate listening clock
    // this requires time zone data even more than other queries
    // XXX need to pass this
    let tzname = Intl.DateTimeFormat().resolvedOptions().timeZone
    console.log(`querying listening data for tz:${tzname}`)

    let p3 = fetch(listeningClockUrl)
        .then(response => response.json())
        .then(data => {
            console.log(`got ${data.length} hours from json call`)

            console.log(data)

            var ctx = document.getElementById('myChart');
            currentValues = data.map(x => x.count)
            averageValues = data.map(x => x.avgCount)

            populateListeningClock(ctx,
                'Apr 2019 Listening Clock', // XXX
                currentValues,
                averageValues);
        })
        .catch(error => {
            console.log("!!! error getting listening clock data")
            console.log(error)
        });

    // to avoid race, wait for both data loading promises to complete
    // before incrementing our hacky demo counter
    Promise.all([p1, p2, p3]).then(vals => {
        console.log("populatePage completed")
        window.populatePageCount += 1;
    })
}

function populateTrackList(tableDom, trackData) {
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

function populateArtistList(tableDom, artistData) {
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

function populateListeningClock(chartDom, title, currentValues, averageValues) {
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

// jquery-like helpers
function empty(domElt) {
    while (domElt.firstChild) {
        domElt.removeChild(domElt.firstChild);
    }
}

function randElt(arr) {
    return arr[Math.floor(Math.random() * arr.length)];
}