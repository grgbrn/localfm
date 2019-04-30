-- confirm that sqlite is storing the epoch correctly
select id, title, artist, album, dt, CAST(strftime('%s', dt) as integer) as uts
from lastfm_activity order by dt desc limit 10;

-- total tracks scrobbled by month
select strftime('%Y-%m', dt), count(*)
from lastfm_activity
group by 1
order by 1;

-- most popular artists of 2018
select artist, count(*) as c
from lastfm_activity
where strftime('%Y', dt)='2018'
group by artist having c > 20 order by c desc;

-- most popular albums of 2019
select artist, album, count(*) as c
from lastfm_activity
where strftime('%Y', dt)='2019'
group by artist, album having c > 20 order by c desc;

-- lastfm "artists" report
select artist, count(*) as c
from lastfm_activity
where dt >= '2019-03-01' and dt < '2019-04-01'
group by artist having c > 20 order by c desc;

-- lastfm "albums" report
select artist, album, count(*) as c
from lastfm_activity
where dt >= '2019-03-01' and dt < '2019-04-01'
group by artist, album having c > 20 order by c desc;

-- lastfm "tracks" report
select title, artist, count(*) as c
from lastfm_activity
where dt >= '2019-03-01' and dt < '2019-04-01'
group by title, artist order by c desc, artist
limit 20;


-- simple month histogram for a single artist
-- http://www.wagonhq.com/sql-tutorial/creating-a-histogram-sql
select strftime('%m', dt) as month,
count(*) as count
from lastfm_activity
where lastfm_activity.artist = 'Panda Bear'
and strftime('%Y', dt)='2018'
group by 1
order by 1;


-- use a subquery to find top 10 artists of a year
-- and generate a sparse histogram from that
select a.artist,
  strftime('%m', dt) as month,
  count(*) as count
from
(select artist, count(*) as c
 from lastfm_activity
 where strftime('%Y', dt)='2018'
 group by artist order by c desc limit 10) top
join lastfm_activity a on a.artist = top.artist
where
strftime('%Y', a.dt)='2018'
group by 1, 2
order by 1, 2;
