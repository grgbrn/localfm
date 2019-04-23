CREATE TABLE lastfm_activity (
	id INTEGER NOT NULL,
	doc JSON,
	created DATETIME,
	artist VARCHAR(255),
	album VARCHAR(255),
	title VARCHAR(255),
	dt DATETIME,
	PRIMARY KEY (id)
);