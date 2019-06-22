const monthlyTrackUrl = "/data/monthlyTracks"
const monthlyArtistUrl = "/data/monthlyArtists"

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
    // XXX this data should theoretically come from fake json call too
    var ctx = document.getElementById('myChart');
    currentValues = [6, 4, 1, 2, 11, 6, 4, 2, 28, 73, 116, 113, 100, 69, 81, 79, 79, 46, 19, 36, 36, 25, 19, 1];
    averageValues = [7, 4, 9, 8, 4, 3, 21, 18, 22, 47, 36, 51, 59, 63, 79, 83, 123, 120, 87, 77, 90, 87, 65, 25];

    if (window.populatePageCount % 2 == 1) {
        currentValues = currentValues.reverse()
        averageValues = averageValues.reverse()
    }

    populateListeningClock(ctx,
        'Apr 2019 Listening Clock',
        currentValues,
        averageValues);

    // to avoid race, wait for both data loading promises to complete
    // before incrementing our hacky demo counter
    Promise.all([p1, p2]).then(vals => {
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
        td[1].children[0].src = dat.imageUrl; // XXX maybe too fragile to template edits
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
        td[0].children[0].src = dat.imageUrl;
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
