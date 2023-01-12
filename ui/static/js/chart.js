window.refreshListeningChart = function (clockData) {
    // clockData corresponds to clockTemplateData struct in golang
    // attributes: title, label, currentValues, avgValues

    let chartContainer = document.querySelector('.chartcontainer')
    chartContainer.innerHTML = "<canvas id='myChart'></canvas>"

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
                data: clockData.currentValues,
                backgroundColor: 'rgba(0,0,255,0.6)',
                borderColor: 'blue',
                fill: true,
                tension: 0.4,
            }, {
                label: clockData.label,
                data: clockData.avgValues,
            }]
        },
        options: {
            responsive: true,
            tooltips: {
                mode: 'index',
                intersect: false,
            },
            hover: {
                mode: 'nearest',
                intersect: true
            },
            scales: {
                x: {
                    title: {
                        display: true,
                        text: "Hour"
                    }
                },
                y: {
                    title: {
                        display: true,
                        text: "Tracks Played"
                    }
                }
            },
            plugins: {
                title: {
                    display: true,
                    text: clockData.title,
                }
            }
        }
    });
}