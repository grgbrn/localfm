# localfm

Store a copy of your last.fm listening history in a local sqlite database for safe keeping and offline analysis

## Setup

Requires golang with module support (1.11+)

Create a new database:

```
sqlite3 $FOO.db < sqlite_schema.sql
```

## Configuration

Most configuration is done through environment vars. LastFM API keys are required:

```
export LASTFM_USERNAME="$YOURNAME"
export LASTFM_API_KEY="XXXXX"
export LASTFM_API_SECRET="YYYYY"
```

DSN is similar to the format of sqlalchemy and other python tools. Since *localfm* only supports sqlite, it only ever needs to refer to a path:

```
export DSN="sqlite://$FOO.db"
```

## Usage

Should be pretty self-explanatory. Run *localfm* on a newly created database and it will download your entire listening history. Subsequent runs will do incremental updates of new
activity since the new run.
