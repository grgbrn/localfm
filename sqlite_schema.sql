CREATE TABLE activity (
	id INTEGER NOT NULL,

	-- store timestamp in both formats for convenience
	uts INTEGER NOT NULL,
	dt DATETIME,

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

	PRIMARY KEY (id),
	FOREIGN KEY(artist_id) REFERENCES artist(id),
	FOREIGN KEY(album_id) REFERENCES album(id),
	FOREIGN KEY(image_id) REFERENCES image(id)
);

CREATE TABLE artist (
	id integer not null,
	name VARCHAR(255) not null,
	mbid VARCHAR(255),
	PRIMARY KEY (id),
	CONSTRAINT artist_unique UNIQUE (name, mbid)
);

CREATE TABLE album (
	id INTEGER NOT NULL,
	name VARCHAR(255) not null,
	mbid VARCHAR(255),
	PRIMARY KEY (id),
	CONSTRAINT album_unique UNIQUE (name, mbid)
);

CREATE TABLE image (
	id INTEGER NOT NULL,
	url VARCHAR(255),
	PRIMARY KEY (id)
);
