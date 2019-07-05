-- query:TOP_TRACKS
-- params: STARTDATE, ENDDATE, TOPN
-- find the most popular tracks for a time period
select a.artist, a.title, count(*) as plays, group_concat(distinct i.url)
from activity a
left join image i on a.image_id = i.id
where a.dt >= '2019-04-01' and a.dt < '2019-05-01'
group by a.artist, a.title
order by plays desc limit 20;


-- query:TOP_ARTISTS
-- params: STARTDATE, ENDDATE, TOPN
-- find the most popular artists for a time period
-- with album art and play counts
select a.artist, count(*) as plays, group_concat(distinct i.url)
from activity a
join image i on a.image_id = i.id
where a.dt >= '2019-06-01' and a.dt < '2019-07-01'
group by a.artist
order by plays desc limit 20;


-- query:NEW_ARTISTS
-- params: STARTDATE, ENDDATE, TOPN
-- find all artists who were played for the first time
-- in the selected range, with album art and play counts
select a.artist, a.plays, min(a2.dt) initial,
a.images
from
(
  select a.artist, a.artist_id,
  count(*) as plays,
  group_concat(distinct i.url) as images
  from activity a
  join image i on a.image_id = i.id
  where a.dt >= '2019-06-01' and a.dt < '2019-07-01'
  group by a.artist, a.artist_id
  order by plays desc
  limit 20
) a
join activity a2 on
a.artist_id = a2.artist_id
group by a.artist_id
having initial >= '2019-06-01' and initial < '2019-07-01';


-- query:NEW_ARTISTS_ALT (unused)
-- params: STARTDATE, ENDDATE, TOPN
-- gives first & last date range and global play counts
-- for top N artists listened to in a certain period
select a.artist_id,
min(a2.dt) p1,
max(a2.dt) p2,
count(*) as total_plays
from
(
  select artist, artist_id, count(*) as c
  from activity
  where dt >= '2019-06-01' and dt < '2019-07-01'
  group by artist, artist_id
  order by c desc
  limit 20
) a
join activity a2 on
a.artist_id = a2.artist_id
group by a.artist_id;


-- query:ARTIST_LEADERBOARD (unused)
-- params: TOPN
-- show overall play counts and date range for artist popularity
select artist, min(dt) t1, max(dt) t2, count(*) plays
from activity
group by artist
order by plays desc
limit 20;


-- query:LISTENING_CLOCK
-- params: STARTDATE, ENDDATE, TOPN
-- collect play counts by hour over a time period
select strftime('%Y-%m-%d %H:00', dt) as hour, count(*) as c
from activity
where dt >= '2019-04-01' and dt < '2019-05-01'
group by 1
order by 1;
