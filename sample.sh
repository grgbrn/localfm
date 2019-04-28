#!/bin/bash

DBNAME="localfm.db"

export LASTFM_USERNAME="YOUR_USERNAME"
export LASTFM_API_KEY="YOUR_API_KEY"
export LASTFM_API_SECRET="YOUR_SECRET_KEY"

export DSN=sqlite://$DBNAME

if [ -f "$DBNAME" ]; then
    echo "backing up previous database"
    cp $DBNAME $DBNAME.prev
fi

./localfm