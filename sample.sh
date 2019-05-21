#!/bin/bash

DBNAME="localfm.db"

export LASTFM_USERNAME="YOUR_USERNAME"
export LASTFM_API_KEY="YOUR_API_KEY"
export LASTFM_API_SECRET="YOUR_SECRET_KEY"

export DSN="sqlite://$DBNAME"

if [ ! -f "$DBNAME" ]; then
    echo "error! $DBNAME does not exist"
    exit 1
fi

# figure out what shell command to use for checksums
SHACMD=""
case $(uname) in
    "Darwin") SHACMD="shasum";;
    "Linux") SHACMD="sha1sum";;
esac

if [ SHACMD == "" ]; then
    echo "error! can't find shasum command"
    exit 1
fi

# backup the existing database only if no backup exists
# or if a backup exists and differs from the current db
DB_BACKUP="${DBNAME}.prev"
if [ ! -f "$DB_BACKUP" ]; then
    echo "creating initial backup"
    cp $DBNAME $DB_BACKUP
else
    OLD=$($SHACMD $DB_BACKUP | awk '{print $1}')
    NEW=$($SHACMD $DBNAME | awk '{print $1}')

    if [ "$OLD" != "$NEW" ]; then
        echo "rotating backups"
        cp $DBNAME $DB_BACKUP
    fi
fi

./localfm
