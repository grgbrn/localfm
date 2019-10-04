# localfm

Store a copy of your last.fm listening history in a local sqlite database for safe keeping and offline analysis

## Setup

Requires golang with module support (1.11+)

```
go build -o localfm cmd/main.go
```

Put the compiled binary wherever you like. Then create a new database:

```
sqlite3 $FOO.db < sqlite_schema.sql
```

## Configuration

Most configuration is done through environment vars.

DSN is similar to the format of sqlalchemy and other python tools. Since *localfm* only supports sqlite, it only ever needs to refer to a path:

```
export DSN="sqlite://$FOO.db"
```

LastFM API keys are required to retrieve updates:
```
export LASTFM_USERNAME="$YOURNAME"
export LASTFM_API_KEY="XXXXX"
export LASTFM_API_SECRET="YYYYY"
```

The web UI requires a little configuration also:
```
export LOGIN_USER="$YOURNAME"
export LOGIN_PASSWD="$BCRYPT_HASH"
export SESSION_SECRET="$RANDOM_32_BYTES"
```

The bcrypt password can be generated from a *nix shell:
```
htpasswd -bnBC 10 "" password | tr -d ':\n'
```

XXX how to generate the session secret?

last.fm streams occasionally contain repeated entries a few seconds apart
which can affect your metrics.  localfm can be configured to flag these repeated entries and exclude them from it's statistics. To enable this
feature, set a threshold (in seconds) in the environment var `LOCALFM_DUPLICATE_THRESHOLD`.  Any tracks that are repeated
    in this interval or less will be flagged.

```
export LOCALFM_DUPLICATE_THRESHOLD="5"
```



A `sample.sh` script is provided which can be customized.

## Usage

Run *localfm* on a newly created database and it will download your entire listening history. Subsequent runs will do incremental updates of new activity since the last run.
If there's an error, a `checkpoint.json` file should be written that allows the process to resume.