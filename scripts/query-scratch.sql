-- total tracks scrobbled by month
select strftime('%Y-%m', dt), count(*)
from activity
where duplicate=false
group by 1
order by 1;

-- most popular artists of 2018
select artist, count(*) as c
from activity
where strftime('%Y', dt)='2018'
and duplicate=false
group by artist having c > 20 order by c desc;

-- most popular albums of 2019
select artist, album, count(*) as c
from activity
where strftime('%Y', dt)='2019'
and duplicate=false
group by artist, album having c > 20 order by c desc;

-- lastfm "artists" report
select artist, count(*) as c
from activity
where dt >= '2019-03-01' and dt < '2019-04-01'
and duplicate=false
group by artist having c > 20 order by c desc;

-- lastfm "albums" report
select artist, album, count(*) as c
from activity
where dt >= '2019-03-01' and dt < '2019-04-01'
and duplicate=false
group by artist, album having c > 20 order by c desc;

-- lastfm "tracks" report
select title, artist, count(*) as c
from activity
where dt >= '2019-03-01' and dt < '2019-04-01'
and duplicate=false
group by title, artist order by c desc, artist
limit 20;


-- simple month histogram for a single artist
-- http://www.wagonhq.com/sql-tutorial/creating-a-histogram-sql
select strftime('%m', dt) as month,
count(*) as count
from activity
where activity.artist = 'Panda Bear'
and strftime('%Y', dt)='2018'
and duplicate=false
group by 1
order by 1;


-- use a subquery to find top 10 artists of a year
-- and generate a sparse histogram from that
select a.artist,
  strftime('%m', dt) as month,
  count(*) as count
from
(select artist, count(*) as c
 from activity
 where strftime('%Y', dt)='2018'
 and duplicate=false
 group by artist order by c desc limit 10) top
join activity a on a.artist = top.artist
where
strftime('%Y', a.dt)='2018'
and duplicate=false
group by 1, 2
order by 1, 2;

-- "listening clock" query
select strftime('%H', dt) as hour, count(*) as c
from activity
where dt >= '2019-04-01' and dt < '2019-05-01'
and duplicate=false
group by 1
order by 1;

-- "new artists" query
-- only really by play count though
select artist, min(dt) as first, count(*) as plays
from activity
group by artist
having plays > 5 and first >= '2019-05-01'
order by first desc;

-- artists by popularity query
-- version with a join to get the url and concat/distinct
select a.artist, count(*) as c, group_concat(distinct i.url)
from activity a
join image i on a.image_id = i.id
where a.dt >= '2019-05-01' and a.dt < '2019-06-01'
group by a.artist
order by c desc limit 20;

-- a version that uses distinct to filter out a single
-- track from a spotify playlist that i played 8 times
-- XXX how do i do this?

-- data for "daily listening" timeline display
select artist, count(*) as c, avg(uts) as uts
from activity
where dt>='2019-05-28' and dt < '2019-05-29'
group by artist order by c desc;

-- doesn't work well when you listen to mixed playlists
-- / needs a way to fill in cluster data for "empty" spots on the graph
select strftime('%H', dt) as h, artist, dt
from activity
where dt>='2019-05-28' and dt < '2019-05-29'
order by dt;


-- start with artists present in dataset for the longest span
select
  l.artist, l.plays, l.t2 - l.t1 as r, l.d1, l.d2
from (
select artist, min(uts) t1, max(uts) t2, min(dt) d1, max(dt) d2, count(*) plays
from activity
group by artist
order by plays desc) l
order by r desc limit 20;

-- bucket by... week?
-- should give me a sparse list for sparklines
select strftime('%Y-%W', dt) as week, count(*)
from activity
where artist='Monster Rally'
group by 1
order by 1;

-- good test data for sparklines
where artist='Sleater-Kinney'
where artist='Juana Molina'
where artist='Monster Rally'

