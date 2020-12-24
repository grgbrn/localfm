CREATE TABLE activity (
	id SERIAL PRIMARY KEY,

	-- store timestamp in both formats for convenience
	uts INTEGER NOT NULL,
	dt TIMESTAMP WITH TIME ZONE,

	title VARCHAR(255),
	mbid VARCHAR(255), -- XXX improve this
	url VARCHAR(1024),

	artist VARCHAR(255),
	artist_id INTEGER,
	album VARCHAR(255),
	album_id INTEGER,

	image_id INTEGER,

	-- lastfm stream has many likely duplicates, flag them
	duplicate BOOLEAN,

	FOREIGN KEY(artist_id) REFERENCES artist(id),
	FOREIGN KEY(album_id) REFERENCES album(id),
	FOREIGN KEY(image_id) REFERENCES image(id)
);

CREATE TABLE artist (
	id SERIAL PRIMARY KEY,
	name VARCHAR(255) not null,
	mbid VARCHAR(255),

	CONSTRAINT artist_unique UNIQUE (name, mbid)
);

CREATE TABLE album (
	id SERIAL PRIMARY KEY,
	name VARCHAR(255) not null,
	mbid VARCHAR(255),

	CONSTRAINT album_unique UNIQUE (name, mbid)
);

CREATE TABLE image (
	id SERIAL PRIMARY KEY,
	url VARCHAR(255) not null
);